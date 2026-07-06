package platform

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func PollProcStat(pid int) error {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return err
	}
	if len(data) == 0 {
		return os.ErrNotExist
	}
	return nil
}

// it polls all procs (/proc) then filters them out to find the root process
func FindRootProcess(name string, procs []*Process) *Process {
	var candidates []*Process
	for _, proc := range procs {
		if strings.Contains(proc.Name, name) {
			candidates = append(candidates, proc)
		}
	}

	if len(candidates) == 0 {
		return nil
	}
	if len(candidates) == 1 {
		return candidates[0]
	}

	parentPids := make(map[int]int)
	for _, c := range candidates {
		parentPids[c.Ppid]++
	}

	for _, c := range candidates {
		if parentPids[c.Pid] > 0 {
			return c
		}
	}

	return candidates[0]
}

func GetBootTime() (int64, error) {
	data, err := os.ReadFile("/proc/stat")
	if err != nil {
		return 0, err
	}
	for line := range bytes.SplitSeq(data, []byte{'\n'}) {
		if bytes.HasPrefix(line, []byte("btime")) {
			fields := bytes.Fields(line)
			if len(fields) < 2 {
				return 0, fmt.Errorf("bad btime line")
			}
			return strconv.ParseInt(string(fields[1]), 10, 64)
		}
	}
	return 0, fmt.Errorf("btime not found")
}

func GetStartTime(proc *Process) error {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", proc.Pid))
	if err != nil {
		return err
	}
	content := bytes.Split(data, []byte{')'})[1]
	fields := bytes.Fields(content)
	for i := range fields {
		if i == 19 {
			bootTime, err := GetBootTime()
			if err != nil {
				return err
			}
			startTicks, _ := strconv.ParseInt(string(fields[19]), 10, 64) // I don't know what is this
			proc.StartTime = time.Unix(bootTime+startTicks/100, (startTicks%100)*10_000_000)
		}
	}
	return nil
}

func PollProc() []*Process {
	var myProcessies []*Process
	err := filepath.WalkDir("/proc/", func(path string, d os.DirEntry, err error) error {
		if strings.HasSuffix(path, "status") {
			parts := strings.Split(path, "/")
			if len(parts) != 4 {
				return nil
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			lines := strings.SplitSeq(string(data), "\n")
			for line := range lines {
				if strings.HasPrefix(line, "Name:") {
					prc := &Process{}
					prc.Name = strings.TrimSpace(strings.Split(line, ":")[1])
					myProcessies = append(myProcessies, prc)
				}
				if strings.HasPrefix(line, "Pid:") {
					fields := strings.Fields(line)
					if len(fields) >= 2 {
						myProcessies[len(myProcessies)-1].Pid, _ = strconv.Atoi(fields[1])
					}
				}
				if strings.HasPrefix(line, "PPid:") {
					fields := strings.Fields(line)
					if len(fields) >= 2 {
						myProcessies[len(myProcessies)-1].Ppid, _ = strconv.Atoi(fields[1])
					}
				}
				if strings.HasPrefix(line, "Tgid:") {
					fields := strings.Fields(line)
					if len(fields) >= 2 {
						myProcessies[len(myProcessies)-1].Tgid, _ = strconv.Atoi(fields[1])
					}
				}
			}
		}
		return nil
	})

	if err != nil {
		fmt.Println(err)
		return nil
	}

	return myProcessies
}
