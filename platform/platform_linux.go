//go:build linux

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

var deviceName = GetDeviceName()

func PollProc(myProcessies *[]*types.Process) {
	uid := os.Getuid()
	err := filepath.WalkDir("/proc/", func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if strings.HasSuffix(path, "status") {
			parts := strings.Split(path, "/")
			if len(parts) != 4 {
				return nil
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return nil
			}

			prc := &types.Process{IsParent: false, DeviceName: deviceName}
			GetOS(prc)

			matchesUID := false

			lines := strings.SplitSeq(string(data), "\n")
			for line := range lines {
				switch {
				case strings.HasPrefix(line, "Name:"):
					prc.Name = strings.TrimSpace(strings.Split(line, ":")[1])

				case strings.HasPrefix(line, "Pid:"):
					if fields := strings.Fields(line); len(fields) >= 2 {
						prc.Pid, _ = strconv.Atoi(fields[1])
					}

				case strings.HasPrefix(line, "PPid:"):
					if fields := strings.Fields(line); len(fields) >= 2 {
						prc.Ppid, _ = strconv.Atoi(fields[1])
					}

				case strings.HasPrefix(line, "Tgid:"):
					if fields := strings.Fields(line); len(fields) >= 2 {
						prc.Tgid, _ = strconv.Atoi(fields[1])
					}

				case strings.HasPrefix(line, "Uid:"):
					// uid line has 4 values: real, effective, saved, filesystem
					if fields := strings.Fields(line); len(fields) >= 2 {
						if ownerUID, err := strconv.Atoi(fields[1]); err == nil && ownerUID == uid {
							matchesUID = true
						}
					}
				}
			}

			if matchesUID {
				GetStartTime(prc)
				*myProcessies = append(*myProcessies, prc)
			}
		}
		return nil
	})
	if err != nil {
		fmt.Println(err)
	}
}
func GetStartTime(proc *types.Process) error {
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

func GetOS(proc *types.Process) {
	proc.OS = "linux"
}

func GetDeviceName() string {
	name, err := os.Hostname()
	if err != nil {
		return ""
	}
	return name
}

// func pollProcStat(pid int) error {
// 	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
// 	if err != nil {
// 		fmt.Println(err)
// 		return err
// 	}
// 	if len(data) == 0 {
// 		return os.ErrNotExist
// 	}
// 	return nil
// }

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
