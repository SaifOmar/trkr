package types

import "time"

const (
	START  = "start"
	END    = "end"
	PAUSE  = "pause"
	RESUME = "resume"
)

type Process struct {
	Name      string
	Pid       int
	Ppid      int
	Tgid      int
	StartTime time.Time
	IsParent  bool
}

type SessionEvent struct {
	Session   *Session
	EventType string // start, end, pause, resume
}

type Session struct {
	StartTime time.Time
	EndTime   time.Time
	Duration  time.Duration

	Proc *Process
}
