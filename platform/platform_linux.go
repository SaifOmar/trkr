package platform

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/SaifOmar/trkr/types"
)

func getBootTime() (int64, error) {
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

func getStartTime(proc *types.Process) error {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", proc.Pid))
	if err != nil {
		return err
	}
	content := bytes.Split(data, []byte{')'})[1]
	fields := bytes.Fields(content)
	for i := range fields {
		if i == 19 {
			bootTime, err := getBootTime()
			if err != nil {
				return err
			}
			startTicks, _ := strconv.ParseInt(string(fields[19]), 10, 64) // I don't know what is this
			proc.StartTime = time.Unix(bootTime+startTicks/100, (startTicks%100)*10_000_000)
		}
	}
	return nil
}

func pollProcStat(pid int) error {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		fmt.Println(err)
		return err
	}
	if len(data) == 0 {
		return os.ErrNotExist
	}
	return nil
}

func watchProc(wC chan *types.SessionEvent, pr *types.Process) {
	err := getStartTime(pr)
	if err != nil {
		pr.StartTime = time.Now()
	}
	ses := &types.Session{StartTime: pr.StartTime}
	wC <- &types.SessionEvent{Session: ses, EventType: types.END}
	for {
		err := pollProcStat(pr.Pid)
		if err != nil {
			ses.EndTime = time.Now()
			ses.Duration = ses.EndTime.Sub(ses.StartTime)
			wC <- &types.SessionEvent{Session: ses, EventType: types.END}
			return
		}
	}
}

func PollProc(myProcessies *[]*types.Process) {
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
					prc := &types.Process{IsParent: false}
					prc.Name = strings.TrimSpace(strings.Split(line, ":")[1])
					*myProcessies = append(*myProcessies, prc)
				}
				if strings.HasPrefix(line, "Pid:") {
					fields := strings.Fields(line)
					if len(fields) >= 2 {
						(*myProcessies)[len(*myProcessies)-1].Pid, _ = strconv.Atoi(fields[1])
					}
				}
				if strings.HasPrefix(line, "PPid:") {
					fields := strings.Fields(line)
					if len(fields) >= 2 {
						(*myProcessies)[len(*myProcessies)-1].Ppid, _ = strconv.Atoi(fields[1])
					}
				}
				if strings.HasPrefix(line, "Tgid:") {
					fields := strings.Fields(line)
					if len(fields) >= 2 {
						(*myProcessies)[len(*myProcessies)-1].Tgid, _ = strconv.Atoi(fields[1])
					}
				}
			}
		}
		return nil
	})

	if err != nil {
		fmt.Println(err)
	}

}
