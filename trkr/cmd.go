package trkr

import (
	"fmt"
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
	WatchList     map[int]types.EventType
	// Store         *store.Store
}

func New(ctx context.Context, processesChan chan []*types.Process, ticker *time.Ticker) *Traker {
	return &Traker{
		ProcessesChan: processesChan,
		Ctx:           ctx,
		Ticker:        ticker,
		mu:            &sync.RWMutex{},
		EventChan:     make(chan types.Event),
		Procceess:     &[]*types.Process{},
		WatchList:     make(map[int]types.EventType),
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
func fillIsParent(processes *[]*types.Process) {
	for _, proc := range *processes {
		root := findRoot(proc, *processes)
		if root == nil {
			continue
		}
		if root.Pid == proc.Pid {
			proc.IsParent = true
		}
	}
}

func (t *Traker) Run() {

	// TODO : should try to fetch from other places there is not reason to panic really
	//

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

// TODO : watch process and send events to the channel on
// CLOSE , PAUSE , RESUME, START, etc
func (t *Traker) Watch(proc *types.Process) {
	for {
		t.mu.Lock()
		p := getProcessByPPID(proc.Ppid, *t.Procceess)
		t.mu.Unlock()
		if p == nil {
			t.EventChan <- types.Event{Type: types.END, Process: proc, Time: time.Now()}
			return
		}
	}
}

func (t *Traker) tick() {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.Procceess = &[]*types.Process{}
	platform.PollProc(t.Procceess)
	fillIsParent(t.Procceess)
	snapshot := make([]*types.Process, len(*t.Procceess))
	copy(snapshot, *t.Procceess)
	t.ProcessesChan <- snapshot

	saved := t.getSavedProcesses()
	if len(saved) <= 0 {
		panic("no saved processes")
	}

	if len(*t.Procceess) <= 0 {
		panic("no processes")
	}

	for _, proc := range saved {
		for _, p := range *t.Procceess {
			if strings.EqualFold(p.Name, proc) {
				fmt.Println("found", p.Name)
				if p.IsParent {
					if t.WatchList[p.Pid] == "" {
						t.EventChan <- types.Event{Type: types.START, Process: p, Time: time.Now()}
					} else {
						continue
					}
				}
			}
		}
	}
}

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
