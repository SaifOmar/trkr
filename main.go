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

func getStartTime(proc *process) {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", proc.pid))
	if err != nil {
		fmt.Println(err)
		return
	}
	content := bytes.Split(data, []byte{')'})[1]
	fields := bytes.Fields(content)
	for i := range fields {
		if i == 19 {
			bootTime, err := getBootTime()
			if err != nil {
				fmt.Println(err)
				return
			}
			startTicks, _ := strconv.ParseInt(string(fields[19]), 10, 64) // I don't know what is this
			proc.startTime = time.Unix(bootTime+startTicks/100, (startTicks%100)*10_000_000)
			return
		}
	}
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
		return err
	}
	if len(data) == 0 {
		return os.ErrNotExist
	}
	return nil
}

func watchProc(wC chan *SessionEvent, pr *process) {
	getStartTime(pr)
	ses := startSession(pr, pr.startTime)
	event := &SessionEvent{session: ses, eventType: START}
	wC <- event
	for {
		err := pollProcStat(pr.pid)
		if err != nil {
			if os.IsNotExist(err) {
				ses.EndTime = time.Now()
				ses.Duration = ses.EndTime.Sub(ses.StartTime)
				event.eventType = END
				wC <- event
				return
			} else {
				panic("An unexpected error occurred")
			}
		}
		time.Sleep(loopInterval)
	}
}

func pollToChan(ch chan []*process) {
	for {
		ch <- pollProc()
		time.Sleep(loopInterval * 4)
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
	dedup := make(map[string]*process)

	go pollToChan(procsChan)

	for {
		select {
		case procs := <-procsChan:
			for _, pr := range procs {
				if pr.name == UserProc {
					if _, ok := dedup[pr.name]; ok {
						continue
					}
					dedup[pr.name] = pr
					go watchProc(sessionEventsChan, pr)
				}
			}
		case event := <-sessionEventsChan:
			switch event.eventType {
			case START:
				SaveProcessDb(event.session.proc, db)
				fmt.Printf(
					"Session started for %s: \n,%8s → %8s (%s)\n",
					event.session.StartTime.Format("15:04:05"),
					event.session.EndTime.Format("15:04:05"),
					event.session.Duration.Round(time.Second),
					event.session.proc.name,
				)
			case END:
				SaveSessionDb(event.session, db)
				fmt.Printf(
					"%8s → %8s (%s)\n",
					event.session.StartTime.Format("15:04:05"),
					event.session.EndTime.Format("15:04:05"),
					event.session.Duration.Round(time.Second),
				)
				delete(dedup, event.session.proc.name)
			}
		}
		time.Sleep(loopInterval)
	}
}
