package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"slices"
	"time"

	"github.com/SaifOmar/trkr/platform"
	_ "modernc.org/sqlite"
)

const loopInterval = time.Millisecond * 500

var watching []string

type SessionEvent struct {
	session   *Session
	eventType string // start, end, pause, resume
}

type Session struct {
	id        int64
	StartTime time.Time
	EndTime   time.Time
	Duration  time.Duration

	proc *platform.Process
}

const (
	START  = "start"
	END    = "end"
	PAUSE  = "pause"
	RESUME = "resume"
)

type Server struct {
	addr string
	port string
	db   *Db
}

var myprocs []*platform.Process

// TODO : take a look into this deep seek changed it
func StartServer(addr, port string, db *Db) *Server {
	s := &Server{addr: addr, port: port, db: db}

	http.HandleFunc("/api/processes", func(w http.ResponseWriter, r *http.Request) {
		myprocs = platform.PollProc()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"processes": myprocs})
	})

	http.HandleFunc("/api/track", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Pid int `json:"pid"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		// get the index of the process in myprocs with the given pid
		i := slices.IndexFunc(myprocs, func(p *platform.Process) bool {
			return p.Pid == req.Pid
		})
		if i == -1 {
			http.Error(w, "process not found", http.StatusNotFound)
			return
		}

		watching = append(watching, myprocs[i].Name)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	http.Handle("/", http.FileServer(http.Dir("clients/frontend")))

	go func() {
		fmt.Println("Starting server on", fmt.Sprintf("%s:%s", addr, port))
		if err := http.ListenAndServe(fmt.Sprintf("%s:%s", addr, port), nil); err != nil {
			fmt.Println("Server error:", err)
		}
	}()

	return s
}

func startSession(proc *platform.Process, t time.Time) *Session {
	return &Session{
		StartTime: t,
		proc:      proc,
	}
}

func watchProc(wC chan *SessionEvent, pr *platform.Process) {
	err := platform.GetStartTime(pr)
	if err != nil {
		pr.StartTime = time.Now()
	}
	ses := startSession(pr, pr.StartTime)
	wC <- &SessionEvent{session: ses, eventType: START}
	for {
		err := platform.PollProcStat(pr.Pid)
		if err != nil {
			ses.EndTime = time.Now()
			ses.Duration = ses.EndTime.Sub(ses.StartTime)
			wC <- &SessionEvent{session: ses, eventType: END}
			return
		}
	}
}

func pollToChan(ch chan []*platform.Process) {
	for {
		ch <- platform.PollProc()
		time.Sleep(loopInterval)
	}
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
	StartServer("127.0.0.1", "8000", db)

	procsChan := make(chan []*platform.Process)
	sessionEventsChan := make(chan *SessionEvent)
	dedup := make(map[int]bool)
	deletedDedup := make(map[int]time.Time)
	dedupThreshhold := time.Second * 5

	go pollToChan(procsChan)

	watching = append(watching, UserProc)
	for {
		select {
		case procs := <-procsChan:
			for _, wi := range watching {
				root := platform.FindRootProcess(wi, procs)
				if root != nil {
					if _, ok := dedup[root.Pid]; ok {
						continue
					}
					if _, ok := deletedDedup[root.Pid]; ok {
						if time.Since(deletedDedup[root.Pid]) < dedupThreshhold {
							continue
						}
					}
					dedup[root.Pid] = true
					go watchProc(sessionEventsChan, root)
				} else {
					fmt.Printf("Could not find root process for %s\n", wi)
				}
			}
		case event := <-sessionEventsChan:
			switch event.eventType {
			case START:
				SaveProcessDb(event.session.proc, db)
				SaveSessionDb(event.session, db)
				fmt.Printf(
					"Session started for %s, at: (%s)\n",
					event.session.proc.Name,
					event.session.StartTime.Format("15:04:05"),
				)
			case END:
				UpdateSessionDb(event.session, db)
				delete(dedup, event.session.proc.Pid)
				deletedDedup[event.session.proc.Pid] = time.Now()
				fmt.Printf(
					"%8s → %8s (%s)\n",
					event.session.StartTime.Format("15:04:05"),
					event.session.EndTime.Format("15:04:05"),
					event.session.Duration.Round(time.Second),
				)
			}
		}
	}
}
