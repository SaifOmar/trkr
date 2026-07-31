//go:build linux

package platform

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/SaifOmar/trkr/types"
)

var deviceName = GetDeviceName()

var cachedProcs = make(map[int]*types.Process)

func GetOS() string {
	return "linux"
}
func PollProc(myProcessies *[]*types.Process) {
	uid := os.Getuid()
	Dir, err := os.ReadDir("/proc")
	if err != nil {
		panic(err)
	}

	// NOTE(future saif) : this is used to remove all processes that closed this tick from the cache, preventing stale cache
	seen := make(map[int]struct{}, len(Dir))

loop:
	for _, f := range Dir {
		if !f.IsDir() {
			continue
		}
		pid, err := strconv.Atoi(f.Name())
		if err != nil {
			continue
		}
		seen[pid] = struct{}{}

		if val, ok := cachedProcs[pid]; ok {
			*myProcessies = append(*myProcessies, val)
			continue
		}

		proc := &types.Process{Pid: pid, IsParent: false, DeviceName: deviceName}

		statusFile, err := os.Open(fmt.Sprintf("/proc/%d/status", pid))
		if err != nil {
			continue
		}

		scanner := bufio.NewScanner(statusFile)
		gotName, gotPid, gotPpid := false, false, false
		for scanner.Scan() {
			line := scanner.Bytes()

			if !gotName && bytes.HasPrefix(line, []byte("Name:")) {
				proc.Name = strings.TrimSpace(string(line[5:]))
				gotName = true
			} else if bytes.HasPrefix(line, []byte("Uid:")) {
				if fields := bytes.Fields(line); len(fields) >= 2 {
					if ownerUID, err := strconv.Atoi(string(fields[1])); err == nil && ownerUID != uid {
						statusFile.Close()
						continue loop
					}
				}
			} else if !gotPid && bytes.HasPrefix(line, []byte("Pid:")) {
				if fields := bytes.Fields(line); len(fields) >= 2 {
					proc.Pid, _ = strconv.Atoi(string(fields[1]))
				}
				gotPid = true
			} else if !gotPpid && bytes.HasPrefix(line, []byte("PPid:")) {
				if fields := bytes.Fields(line); len(fields) >= 2 {
					proc.Ppid, _ = strconv.Atoi(string(fields[1]))
				}
				gotPpid = true
			}

			if gotName && gotPid && gotPpid {
				break
			}
		}
		statusFile.Close()

		proc.OS = GetOS()
		proc.StartTime = GetProcStartTime(pid)
		cachedProcs[pid] = proc
		*myProcessies = append(*myProcessies, proc)
	}

	for pid := range cachedProcs {
		if _, ok := seen[pid]; !ok {
			delete(cachedProcs, pid)
		}
	}
}

func GetProcStartTime(pid int) time.Time {
	// NOTE(saif): don't go back to computing start time from btime we don't need that type of time accuracy
	var StartTime time.Time
	dirInfo, err := os.Stat(fmt.Sprintf("/proc/%d", pid))
	if err != nil {
		return time.Time{}
	}
	if stat, ok := dirInfo.Sys().(*syscall.Stat_t); ok {
		StartTime = time.Unix(stat.Ctim.Sec, stat.Ctim.Nsec)
	}
	return StartTime
}

func GetDeviceName() string {
	name, err := os.Hostname()
	if err != nil {
		return ""
	}
	return name
}
