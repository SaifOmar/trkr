package trkr

import (
	"fmt"
	"log"
	"os"
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
	AutoWatchList []string

	Paused  map[int]bool
	Stopped map[int]bool // stpped manualy
	Watched map[int]types.EventType

	LocalStore *store.Store
}

func New(ctx context.Context, processesChan chan []*types.Process, ticker *time.Ticker, store *store.Store) *Traker {
	return &Traker{
		ProcessesChan: processesChan,
		Ctx:           ctx,
		Ticker:        ticker,
		LocalStore:    store,
		mu:            &sync.RWMutex{},
		Procceess:     &[]*types.Process{},
		EventChan:     make(chan types.Event, 10),
		Watched:       make(map[int]types.EventType),
		Stopped:       make(map[int]bool),
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
	wtf := func(m map[int]types.EventType) {
		f, err := os.OpenFile(
			"testwatched.txt",
			os.O_CREATE|os.O_WRONLY|os.O_TRUNC,
			0644,
		)
		if err != nil {
			log.Fatal(err)
		}
		defer f.Close()
		// t.mu.Lock()
		for k, v := range m {
			p := filterbyPid(k, t.Procceess)
			if p == nil {
				continue
			}
			_, err = fmt.Fprintf(f, "%d %s %s\n", p.Pid, p.Name, v)
			if err != nil {
				log.Fatal(err)
			}
		}
		// t.mu.Unlock()

		if err != nil {
			log.Fatal(err)
		}
	}
	t.getAutoWatchList()
	for {
		select {
		case <-t.Ticker.C:
			t.tick()
			wtf(t.Watched)

		case <-t.Ctx.Done():
			t.Ticker.Stop()
			close(t.ProcessesChan)
			return
		}
	}
}

// NOTE: watch process and send events to the channel on
// CLOSE , PAUSE , RESUME, START, etc
// TODO : PAUSE, RESUME
func (t *Traker) watch(proc *types.Process) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	breaker := false

	for {
		select {
		case <-t.Ctx.Done():
			return
		case <-ticker.C:
			t.mu.Lock()
			val, ok := t.Watched[proc.Pid]
			p := filterbyPid(proc.Pid, t.Procceess)

			var ev types.Event

			if p != nil {
				if t.isProcessActive(p) {
					switch {
					case t.Stopped[proc.Pid]: // stop tracking that process
						delete(t.Watched, proc.Pid)
						breaker = true
					case !ok: // process is not being watched ?? why ?
						t.Watched[proc.Pid] = types.START
						ev = types.Event{Type: types.START, Process: proc, Time: time.Now().UTC()}
					//
					case val == types.END:
						delete(t.Watched, proc.Pid)
						breaker = true
					// TODO : add a timer (maybe user can change it but make it at least 5 mints)
					// and debounce chaging the pause resume with it
					case val == types.START:
						t.Watched[proc.Pid] = types.RUNNING
						ev = types.Event{Type: types.START, Process: proc, Time: time.Now().UTC()}
					case val == types.PAUSE:
						t.Watched[proc.Pid] = types.RUNNING // I can have this be running
						ev = types.Event{Type: types.RESUME, Process: proc, Time: time.Now().UTC()}
					}
				} else {
					if val == types.RUNNING {
						t.Watched[proc.Pid] = types.PAUSE
						ev = types.Event{Type: types.PAUSE, Process: proc, Time: time.Now().UTC()}
					}
				}

			} else {
				switch val {
				// NOTE (saif) : the first case is just a safety fallback for when a process is closed from the server
				case types.END:
					delete(t.Watched, proc.Pid)
					breaker = true
				default:
					t.Watched[proc.Pid] = types.END
					ev = types.Event{Type: types.END, Process: proc, Time: time.Now().UTC()}
				}
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
			return
		}
	}
}

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
					if !t.Stopped[p.Pid] {
						if _, ok := t.Watched[p.Pid]; !ok {
							t.Watched[p.Pid] = types.START
							go t.watch(p)
						}
					}
				}
			}
		}
	}
}

func (t *Traker) getAutoWatchList() {
	v := t.LocalStore.GetAllAutoWatch()
	for _, autoWatch := range v {
		t.AutoWatchList = append(t.AutoWatchList, autoWatch.Name)
	}
}

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
	_, ok := t.Watched[pid]
	if ok {
		t.Stopped[pid] = true
		t.Watched[pid] = types.END
	}
	t.mu.Unlock()

	if ok {
		select {
		case t.EventChan <- types.Event{Type: types.END, Process: p, Time: time.Now().UTC()}:
		case <-t.Ctx.Done():
		}
	}
}

func (t *Traker) AddManualWatch(pid int) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	p := filterbyPid(pid, t.Procceess)
	if p == nil {
		return false
	}

	if t.Stopped[p.Pid] {
		delete(t.Stopped, p.Pid)
	}

	val, ok := t.Watched[p.Pid]
	if !ok || val == types.END {
		delete(t.Watched, p.Pid)
		t.Watched[p.Pid] = types.START
		go t.watch(p)
		return true
	}

	return false
}
func (t *Traker) isProcessActive(proc *types.Process) bool {
	return platform.IsWindowFocused(proc.Pid)
}
