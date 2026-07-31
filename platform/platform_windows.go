//go:build windows

package platform

import (
	"os"
	"strings"
	"time"
	"unsafe"

	"github.com/SaifOmar/trkr/types"
	"golang.org/x/sys/windows"
)

var deviceName = GetDeviceName()

func PollProc(processes *[]*types.Process) {
	// Note(saif): we don't really need to cache here
	// all tests on windows shows almost 0 cpu usage, but maybe still profile this
	hwin, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		panic(err)
	}
	defer windows.CloseHandle(hwin)
	var ProcessEnetry32 windows.ProcessEntry32
	ProcessEnetry32.Size = uint32(unsafe.Sizeof(windows.ProcessEntry32{}))
	for {
		err := windows.Process32Next(hwin, &ProcessEnetry32)
		if err != nil {
			if err == windows.ERROR_NO_MORE_FILES {
				break
			}
			panic(err)
		}
		name := strings.TrimSuffix(windows.UTF16ToString(ProcessEnetry32.ExeFile[:]), ".exe")
		p := &types.Process{Name: name, Pid: int(ProcessEnetry32.ProcessID), Ppid: int(ProcessEnetry32.ParentProcessID), DeviceName: deviceName}
		p.StartTime = GetStartTime(int(ProcessEnetry32.ProcessID))
		p.OS = GetOS()
		*processes = append(*processes, p)
	}
}

func GetStartTime(pid int) time.Time {
	var StartTime time.Time
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		return StartTime
	}
	defer windows.CloseHandle(h)

	var creation, exit, kernel, user windows.Filetime
	if err := windows.GetProcessTimes(h, &creation, &exit, &kernel, &user); err != nil {
		return StartTime
	}

	StartTime = time.Unix(0, creation.Nanoseconds())
	return StartTime
}

func GetDeviceName() string {
	name, err := os.Hostname()
	if err != nil {
		return ""
	}
	return name
}

func GetOS() string {
	return "windows"
}
