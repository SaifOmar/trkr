package main

import (
	"fmt"
	"time"

	"github.com/SaifOmar/trkr/platform"
	"github.com/SaifOmar/trkr/types"
	_ "modernc.org/sqlite"
)

const loopInterval = time.Millisecond * 500

func findRoot(proc *types.Process, child *types.Process, processes []*types.Process) *types.Process {
	getProcessByPPID := func(ppid int, procs []*types.Process) *types.Process {
		for _, proc := range procs {
			if proc.Pid == ppid {
				return proc
			}
		}
		return nil
	}

	// this will skip first process which is system process on windows
	// TODO : look how this behaves on linux
	if proc.Pid == 0 {
		return proc
	}

	if proc.Name != child.Name {
		return child
	}

	parent := getProcessByPPID(proc.Ppid, processes)

	if parent == nil {
		return proc
	}

	return findRoot(parent, child, processes)
}

func main() {
	processes := make([]*types.Process, 0)
	platform.PollProc(&processes)

	// TODO : should optimize this
	for _, proc := range processes {
		root := findRoot(proc, proc, processes)
		if root.Pid == proc.Pid {
			proc.IsParent = true
		}
		fmt.Println(proc)
	}
}
