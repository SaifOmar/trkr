package trkr

import (
	"fmt"
	"strings"
	"sync"

	"context"
	"time"

	"github.com/SaifOmar/trkr/platform"
	"github.com/SaifOmar/trkr/store"
	"github.com/SaifOmar/trkr/types"
)

type Traker struct {
	mu            *sync.RWMutex
	Procceess     *[]*types.Process
	Ctx           context.Context
	Ticker        *time.Ticker
	ProcessesChan chan []*types.Process
	EventChan     chan types.Event
	*WatchList
	AutoWatchList []string

	LocalStore *store.Store
	watchDone  chan struct{}
}

type WatchList struct {
	mu      *sync.RWMutex
	Watched map[int]types.EventType
}

func New(ctx context.Context, processesChan chan []*types.Process, ticker *time.Ticker, store *store.Store) *Traker {
	return &Traker{
		ProcessesChan: processesChan,
		Ctx:           ctx,
		Ticker:        ticker,
		mu:            &sync.RWMutex{},
		EventChan:     make(chan types.Event, 10),
		Procceess:     &[]*types.Process{},
		WatchList:     &WatchList{mu: &sync.RWMutex{}, Watched: make(map[int]types.EventType)},
		LocalStore:    store,
		watchDone:     make(chan struct{}, 100),
	}
}

func getProcessByPPID(ppid int, procs []*types.Process) *types.Process {
	for _, proc := range procs {
		if proc.Pid == ppid {
			return proc
		}
	}
	return nil
}

// walk down the tree by getting the parnt id (ppid on the proccess) find that process and repeat
// a process is root only if it has the same name of the process but it's parent have not the same name
func findRoot(proc *types.Process, processes []*types.Process) *types.Process {
	if proc == nil {
		return nil
	}

	if proc.Ppid == 0 {
		return proc
	}

	parent := getProcessByPPID(proc.Ppid, processes)

	if parent == nil {
		return proc
	}

	if parent.Name != proc.Name {
		return proc
	}

	return findRoot(parent, processes)
}

// TODO : should optimize this
func (t *Traker) fillIsParent() {
	for _, proc := range *t.Procceess {
		root := findRoot(proc, *t.Procceess)
		if root == nil {
			continue
		}
		if root.Pid == proc.Pid {
			proc.IsParent = true
		}
	}
}

func (t *Traker) Run() {
	t.getAutoWatchList()
	for {
		select {
		case <-t.Ticker.C:
			fmt.Println("tick")
			t.tick()
		case <-t.Ctx.Done():
			t.Ticker.Stop()
			close(t.ProcessesChan)
			fmt.Println("returning")
			return
		}
	}
}

// NOTE: watch process and send events to the channel on
// CLOSE , PAUSE , RESUME, START, etc
// TODO : PAUSE, RESUME
func (t *Traker) Watch(proc *types.Process) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	breaker := false

	for {
		select {
		case <-t.Ctx.Done():
			return

		case <-ticker.C:
			t.mu.Lock()
			val, ok := t.WatchList.Watched[proc.Pid]
			p := filterbyPid(proc.Pid, t.Procceess)
			err := platform.GetStartTime(proc)

			var ev types.Event

			if p != nil {
				if err != nil {
					fmt.Println(err)
				}
				if !ok {
					t.WatchList.Watched[proc.Pid] = types.START
					ev = types.Event{Type: types.START, Process: proc, Time: time.Now().UTC()}
				} else {
					if val == types.END {
						t.WatchList.Watched[proc.Pid] = ""
						breaker = true
					}
				}
			} else {
				if val != types.END {
					t.WatchList.Watched[proc.Pid] = types.END
					ev = types.Event{Type: types.END, Process: proc, Time: time.Now().UTC()}
				}
				// for clean up so the goroutine doesn't keep running for ever
				if val == types.END {
					t.WatchList.Watched[proc.Pid] = ""
				}
				breaker = true
			}

			t.mu.Unlock()

			if ev.Type != "" {
				select {
				case t.EventChan <- ev:
				case <-t.Ctx.Done():
					return
				}
			}
		}

		if breaker {
			fmt.Println("breaker")
			return
		}
	}
}

// func writeToTestFile(p *types.Process) {
// 	cwd, err := os.Getwd()
// 	if err != nil {
// 		fmt.Println(err)
// 		return
// 	}
//
// 	fileName := filepath.Join(cwd, "cmd.txt")
//
// 	f, err := os.OpenFile(fileName, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
// 	if err != nil {
// 		fmt.Println(err)
// 		return
// 	}
// 	defer f.Close()
//
// 	if _, err := fmt.Fprintf(f, "%+v\n", p); err != nil {
// 		fmt.Println(err)
// 	}
// }

func filterbyPid(pid int, procs *[]*types.Process) *types.Process {
	var p *types.Process
	for _, proc := range *procs {
		if proc.Pid == pid {
			p = proc
		}
	}
	return p
}

func (t *Traker) tick() {
	t.mu.Lock()
	t.Procceess = &[]*types.Process{}
	platform.PollProc(t.Procceess)
	t.fillIsParent()
	snapshot := make([]*types.Process, len(*t.Procceess))
	copy(snapshot, *t.Procceess)
	t.mu.Unlock()

	select {
	case t.ProcessesChan <- snapshot:
	case <-t.Ctx.Done():
		return
	}

	t.mu.Lock()
	defer t.mu.Unlock()
	if len(t.AutoWatchList) <= 0 {
		return
	}

	for _, proc := range t.AutoWatchList {
		for _, p := range *t.Procceess {
			if strings.EqualFold(p.Name, proc) {
				if p.IsParent {
					t.WatchList.mu.Lock()
					if t.WatchList.Watched[p.Pid] == "" {
						go t.Watch(p)
					}
					t.WatchList.mu.Unlock()
				}
			}
		}
	}
}

// TODO : look into the store for this and return the slice
func (t *Traker) getAutoWatchList() {
	v := t.LocalStore.GetAllAutoWatch()
	for _, autoWatch := range v {
		t.AutoWatchList = append(t.AutoWatchList, autoWatch.Name)
	}
}

// func (t *Traker) closeAll() {
// 	close(t.ProcessesChan)
// }

func FilterWithName(name string, procs *[]*types.Process) *types.Process {
	for _, proc := range *procs {
		if strings.EqualFold(proc.Name, name) {
			return proc
		}
	}
	return nil

}

func (t *Traker) AddAutoWatch(name string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	for _, n := range t.AutoWatchList {
		if strings.EqualFold(n, name) {
			return
		}
	}
	t.AutoWatchList = append(t.AutoWatchList, name)
}

func (t *Traker) RemoveAutoWatch(name string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	for i, n := range t.AutoWatchList {
		if strings.EqualFold(n, name) {
			t.AutoWatchList = append(t.AutoWatchList[:i], t.AutoWatchList[i+1:]...)
			return
		}
	}
}
func getProcessByPid(pid int, procs *[]*types.Process) *types.Process {
	for _, proc := range *procs {
		if proc.Pid == pid {
			return proc
		}
	}
	return nil
}

func (t *Traker) StopWatching(pid int) {
	t.mu.Lock()
	p := getProcessByPid(pid, t.Procceess)
	fmt.Printf("found this proc: %+v\n", p)
	if _, ok := t.WatchList.Watched[pid]; ok {
		t.WatchList.Watched[pid] = types.END
		t.EventChan <- types.Event{Type: types.END, Process: p, Time: time.Now().Local()}
	}
	t.mu.Unlock()
}

// func (t *Traker) StopWatchingByName(name string) []*types.Process {
// 	t.mu.Lock()
// 	defer t.mu.Unlock()
//
// 	var stopped []*types.Process
// 	for _, proc := range *t.Procceess {
// 		if strings.EqualFold(proc.Name, name) {
// 			if _, ok := t.WatchList.Watched[proc.Pid]; ok {
// 				t.WatchList.Watched[proc.Pid] = types.END
// 				stopped = append(stopped, proc)
// 			}
// 		}
// 	}
// 	return stopped
// }

func (t *Traker) AddManualWatch(pid int) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	p := filterbyPid(pid, t.Procceess)
	if p == nil {
		return false
	}
	t.WatchList.mu.Lock()
	defer t.WatchList.mu.Unlock()
	if t.WatchList.Watched[p.Pid] == "" {
		go t.Watch(p)
		return true
	}
	return false
}
