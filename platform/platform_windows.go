package platform

import (
	"strings"
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
