package main

import (
	"bytes"
	"database/sql"
	"fmt"
	"log"
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

type Session struct {
	StartTime time.Time
	EndTime   time.Time
	Duration  time.Duration

	proc process
}
type Db struct {
	conn *sql.DB
}

func saveSessionDb(ses *Session, db *Db) {
	sqlStmt := `
		INSERT INTO processes (name, pid, ppid, tgid, start_time, end_time, duration_seconds)
		VALUES (?, ?, ?, ?, ?, ?, ?);
	`
	_, err := db.conn.Exec(sqlStmt, ses.proc.name, ses.proc.pid, ses.proc.ppid, ses.proc.tgid, ses.StartTime.Format("2006-01-02 15:04:05"), ses.EndTime.Format("2006-01-02 15:04:05"), ses.Duration.Seconds())
	if err != nil {
		fmt.Println(err)
		return
	}

}
func initDb(path string) *Db {
	conn, err := sql.Open("sqlite", path)
	if err != nil {
		fmt.Println(err)
		return nil
	}
	if err := conn.Ping(); err != nil {
		fmt.Println(err)
		return nil
	}

	sqlStmt := `
        CREATE TABLE IF NOT EXISTS processes (
            id INTEGER PRIMARY KEY,
            name TEXT,
            pid INTEGER,
            ppid INTEGER,
            tgid INTEGER,
            start_time TEXT,
            end_time TEXT,
            duration_seconds INTEGER
        );
    `
	if _, err := conn.Exec(sqlStmt); err != nil {
		fmt.Println(err)
		return nil
	}

	log.Println("Connected to database")
	return &Db{conn: conn}
}
func main() {
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Println(err)
		return
	}
	log.Println("Current working directory:", cwd)

	db := initDb(cwd + "/trkr.db")
	defer db.conn.Close()

	rows, err := db.conn.Query("SELECT * FROM processes")
	if err != nil {
		fmt.Println(err)
		return
	}
	rows.Close()

	// for rows.Next() {
	// 	var id int
	// 	var name string

	// 	if err := rows.Scan(&id, &name); err != nil {
	// 		fmt.Println(err)
	// 		return
	// 	}

	// 	fmt.Println(id, name)
	// }

	if err := rows.Err(); err != nil {
		fmt.Println(err)
		return
	}

	myProcessies := []process{}
	zed := "aether"

	err = filepath.WalkDir("/proc/", func(path string, d os.DirEntry, err error) error {
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
					prc := process{}
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

			// logging
			// log := func() error {
			// writePath := "/home/saif/Dev/dump.txt"
			// 	file, err := os.OpenFile(
			// 		writePath,
			// 		os.O_CREATE|os.O_WRONLY|os.O_APPEND,
			// 		0644,
			// 	)
			// 	if err != nil {
			// 		return err
			// 	}
			// 	defer file.Close()

			// 	_, err = file.Write(data)
			// 	if err != nil {
			// 		return err
			// 	}
			// 	return nil
			// }
			// log()
		}
		return nil
	})

	if err != nil {
		fmt.Println(err)
		return
	}

	var mainThread process
	findRootProcessByname := func(procs []process, procName string) process {
		for _, p := range procs {
			if p.name == procName {
				return p
			}
		}
		return process{}
	}

	findRootProcessBypid := func(procs []process, pid int) process {
		for _, p := range procs {
			if p.pid == pid {
				return p
			}
		}
		return process{}
	}

	getBootTime := func() (int64, error) {
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
	getStartTime := func(pid int, proc *process) {
		data, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
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

	mainThread = findRootProcessByname(myProcessies, zed)
	anotherThread := findRootProcessBypid(myProcessies, 1802)
	getStartTime(mainThread.pid, &mainThread)
	getStartTime(anotherThread.pid, &anotherThread)

	startSession := func(procName string) *Session {
		return &Session{
			StartTime: time.Now(),
			proc:      findRootProcessByname(myProcessies, procName),
		}
	}

	readCgroup := func(pid int) ([]byte, error) {
		data, err := os.ReadFile(fmt.Sprintf("/proc/%d/cgroup", pid))
		if err != nil {
			return nil, err
		}
		return data, nil
	}
	data, err := readCgroup(mainThread.pid)
	fmt.Println(string(data))
	ses := startSession(zed)

	trackSession := func(ses *Session) {
		for {
			data, err := readCgroup(ses.proc.pid)
			if err != nil {
				if os.IsNotExist(err) {
					ses.EndTime = time.Now()
					ses.Duration = ses.EndTime.Sub(ses.StartTime)
					return
				}
				panic(err)
			}
			fmt.Println(string(data))
			time.Sleep(loopInterval)
		}
	}
	trackSession(ses)
	saveSessionDb(ses, db)

	fmt.Printf(
		"%8s → %8s (%s)\n",
		ses.StartTime.Format("15:04:05"),
		ses.EndTime.Format("15:04:05"),
		ses.Duration.Round(time.Second),
	)
}
