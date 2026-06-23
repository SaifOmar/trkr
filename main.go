package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

const loopInterval = time.Millisecond * 500

type process struct {
	name      string
	pid       int
	ppid      int
	tgid      int
	startTime time.Time
}

type SessionEvent struct {
	session   *Session
	eventType string // start, end, pause, resume
}

type Session struct {
	StartTime time.Time
	EndTime   time.Time
	Duration  time.Duration

	proc *process
}

const (
	START  = "start"
	END    = "end"
	PAUSE  = "pause"
	RESUME = "resume"
)

// it poll all procs (/proc) then filters them out to find the root process
func findRootProcess(name string, procs []*process) *process {
	var candidates []*process
	for _, proc := range procs {
		if strings.Contains(proc.name, name) {
			candidates = append(candidates, proc)
		}
	}

	if len(candidates) == 0 {
		return nil
	}
	if len(candidates) == 1 {
		return candidates[0]
	}

	parentPids := make(map[int]int) // pid -> how many times it appears as a ppid
	for _, c := range candidates {
		parentPids[c.ppid]++
	}

	for _, c := range candidates {
		if parentPids[c.pid] > 0 {
			return c
		}
	}

	return candidates[0] // fallback
}

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

func getStartTime(proc *process) error {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", proc.pid))
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
			proc.startTime = time.Unix(bootTime+startTicks/100, (startTicks%100)*10_000_000)
		}
	}
	return nil
}

func startSession(proc *process, t time.Time) *Session {
	return &Session{
		StartTime: t,
		proc:      proc,
	}
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

func watchProc(wC chan *SessionEvent, pr *process) {
	err := getStartTime(pr)
	if err != nil {
		pr.startTime = time.Now()
	}
	ses := startSession(pr, pr.startTime)
	wC <- &SessionEvent{session: ses, eventType: START}
	for {
		err := pollProcStat(pr.pid)
		if err != nil {
			ses.EndTime = time.Now()
			ses.Duration = ses.EndTime.Sub(ses.StartTime)
			wC <- &SessionEvent{session: ses, eventType: END}
			return
		}
	}
}

func pollToChan(ch chan []*process) {
	for {
		ch <- pollProc()
		time.Sleep(loopInterval)
	}
}

func pollProc() []*process {
	var myProcessies []*process
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
					prc := &process{}
					prc.name = strings.TrimSpace(strings.Split(line, ":")[1])
					myProcessies = append(myProcessies, prc)
				}
				if strings.HasPrefix(line, "Pid:") {
					fields := strings.Fields(line)
					if len(fields) >= 2 {
						myProcessies[len(myProcessies)-1].pid, _ = strconv.Atoi(fields[1])
					}
				}
				if strings.HasPrefix(line, "PPid:") {
					fields := strings.Fields(line)
					if len(fields) >= 2 {
						myProcessies[len(myProcessies)-1].ppid, _ = strconv.Atoi(fields[1])
					}
				}
				if strings.HasPrefix(line, "Tgid:") {
					fields := strings.Fields(line)
					if len(fields) >= 2 {
						myProcessies[len(myProcessies)-1].tgid, _ = strconv.Atoi(fields[1])
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

func main() {
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Println(err)
		return
	}

	if len(os.Args) < 2 {
		fmt.Println("Error: Please provide a process name to monitor")
		os.Exit(0)
		return
	}

	UserProc := os.Args[1]
	if UserProc == "" {
		fmt.Println("no process name specified")
		os.Exit(0)
		return
	}

	db := InitDb(cwd + "/trkr.db")
	defer db.conn.Close()

	procsChan := make(chan []*process)
	sessionEventsChan := make(chan *SessionEvent)
	dedup := make(map[int]bool)

	go pollToChan(procsChan)

	wathcing := []string{"zed-editor", "aether", UserProc}

	for {
		select {
		case procs := <-procsChan:
			for _, wi := range wathcing {
				root := findRootProcess(wi, procs)
				if root != nil {
					if _, ok := dedup[root.pid]; ok {
						continue
					}
					dedup[root.pid] = true
					go watchProc(sessionEventsChan, root)
				} else {
					fmt.Printf("Could not find root process for %s\n", wi)
				}
			}
		case event := <-sessionEventsChan:
			switch event.eventType {
			case START:
				SaveProcessDb(event.session.proc, db)
				fmt.Printf(
					"Session started for %s, at: (%s)\n",
					event.session.proc.name,
					event.session.StartTime.Format("15:04:05"),
				)
			case END:
				SaveSessionDb(event.session, db)
				fmt.Printf(
					"%8s → %8s (%s)\n",
					event.session.StartTime.Format("15:04:05"),
					event.session.EndTime.Format("15:04:05"),
					event.session.Duration.Round(time.Second),
				)
				delete(dedup, event.session.proc.pid)
			}
		}
	}
}
