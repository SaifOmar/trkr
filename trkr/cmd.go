package trkr

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"context"
	"time"

	"github.com/SaifOmar/trkr/platform"
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
	// Store         *store.Store
}

type WatchList struct {
	mu      *sync.RWMutex
	Watched map[int]types.EventType
}

func New(ctx context.Context, processesChan chan []*types.Process, ticker *time.Ticker) *Traker {
	return &Traker{
		ProcessesChan: processesChan,
		Ctx:           ctx,
		Ticker:        ticker,
		mu:            &sync.RWMutex{},
		EventChan:     make(chan types.Event),
		Procceess:     &[]*types.Process{},
		WatchList:     &WatchList{mu: &sync.RWMutex{}, Watched: make(map[int]types.EventType)},
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
	for {
		select {
		case <-t.Ticker.C:
			fmt.Println("tick")
			t.tick()
		case <-t.Ctx.Done():
			t.closeAll()
		}
	}
}

// NOTE: watch process and send events to the channel on
// CLOSE , PAUSE , RESUME, START, etc
// TODO : PAUSE, RESUME
func (t *Traker) Watch(proc *types.Process) {
	sleepAnUnlock := func() {
		t.mu.Unlock()
		time.Sleep(time.Second * 2)
	}
	for {
		t.mu.Lock()
		// look what was the lastt event for this process
		val, ok := t.WatchList.Watched[proc.Pid]
		p := filterbyPid(proc.Pid, t.Procceess)
		err := platform.GetStartTime(proc)
		if p != nil {
			if err != nil {
				fmt.Println(err)
			}
			if !ok {
				t.WatchList.Watched[proc.Pid] = types.START
				t.EventChan <- types.Event{Type: types.START, Process: proc, Time: proc.StartTime}
			}
		} else {
			if val != types.END {
				t.WatchList.Watched[proc.Pid] = types.END
				t.EventChan <- types.Event{Type: types.END, Process: proc, Time: time.Now().UTC()}
			}

		}
		writeToTestFile(proc)
		sleepAnUnlock()
	}
}

func writeToTestFile(p *types.Process) {
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Println(err)
		return
	}

	fileName := filepath.Join(cwd, "cmd.txt")

	f, err := os.OpenFile(fileName, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer f.Close()

	if _, err := fmt.Fprintf(f, "%+v\n", p); err != nil {
		fmt.Println(err)
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
	defer t.mu.Unlock()
	t.Procceess = &[]*types.Process{}
	platform.PollProc(t.Procceess)
	t.fillIsParent()
	snapshot := make([]*types.Process, len(*t.Procceess))
	copy(snapshot, *t.Procceess)
	t.ProcessesChan <- snapshot

	saved := t.getSavedProcesses()
	if len(saved) <= 0 {
		return
	}

	for _, proc := range saved {
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
func (t *Traker) getSavedProcesses() []string {
	return []string{"firefox", "chrome", "vscode", "nvim", "zed-editor"}
}

func (t *Traker) Save(p *types.Process) {
	// t.mu.Lock()
	// defer t.mu.Unlock()
	// t.Store.Save(p)
}
func (t *Traker) closeAll() {
	close(t.EventChan)
	close(t.ProcessesChan)
}

func FilterWithName(name string, procs *[]*types.Process) *types.Process {
	for _, proc := range *procs {
		if strings.EqualFold(proc.Name, name) {
			return proc
		}
	}
	return nil

}
