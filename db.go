package main

import (
	"database/sql"
	"fmt"
	"log"

	"github.com/SaifOmar/trkr/types"
	_ "modernc.org/sqlite"
)

type Db struct {
	conn *sql.DB
}

func SaveProcessDb(proc *types.Process, db *Db) {
	sqlStmt := `
		INSERT INTO processes (name, pid, ppid, tgid, start_time)
		VALUES (?, ?, ?, ?, ?);
	`
	_, err := db.conn.Exec(sqlStmt, proc.Name, proc.Pid, proc.Ppid, proc.Tgid, proc.StartTime.Format("2006-01-02 15:04:05"))
	if err != nil {
		fmt.Println(err)
		return
	}
}

func SaveSessionDb(ses *types.Session, db *Db) {
	sqlStmt := `
		INSERT INTO sessions(process_id, start_time, end_time, duration_seconds)
		VALUES (?, ?, ?, ?);
	`
	_, err := db.conn.Exec(sqlStmt, ses.Proc.Pid, ses.StartTime.Format("2006-01-02 15:04:05"), ses.EndTime.Format("2006-01-02 15:04:05"), ses.Duration.Seconds())
	if err != nil {
		fmt.Println(err)
		return
	}
}

func InitDb(path string) *Db {
	conn, err := sql.Open("sqlite", path)
	if err != nil {
		fmt.Println(err)
		return nil
	}
	if err := conn.Ping(); err != nil {
		fmt.Println(err)
		return nil
	}
	_, err = conn.Exec(`
CREATE TABLE IF NOT EXISTS processes (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT,
    pid INTEGER,
    ppid INTEGER,
    tgid INTEGER,
    start_time TEXT
)
`)
	if err != nil {
		log.Fatal(err)
	}

	_, err = conn.Exec(`
CREATE TABLE IF NOT EXISTS sessions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    process_id INTEGER,
    start_time TEXT,
    end_time TEXT,
    duration_seconds INTEGER,
    FOREIGN KEY(process_id) REFERENCES processes(id)
)
`)
	if err != nil {
		log.Fatal(err)
	}

	log.Println("Connected to database")
	return &Db{conn: conn}
}
