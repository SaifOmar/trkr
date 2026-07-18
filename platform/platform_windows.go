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

func PollProc(processes *[]*types.Process) {
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
		*processes = append(*processes, &types.Process{Name: name, Pid: int(ProcessEnetry32.ProcessID), Ppid: int(ProcessEnetry32.ParentProcessID)})
	}

}
func GetStartTime(proc *types.Process) error {
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(proc.Pid))
	if err != nil {
		return err
	}
	defer windows.CloseHandle(h)

	var creation, exit, kernel, user windows.Filetime
	if err := windows.GetProcessTimes(h, &creation, &exit, &kernel, &user); err != nil {
		return err
	}

	proc.StartTime = time.Unix(0, creation.Nanoseconds())
	return nil
}

func GetDeviceName() (string, error) {
	name, err := os.Hostname()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(name), nil
}

func GetOS(proc *types.Process) {
	proc.OS = "windows"
}
