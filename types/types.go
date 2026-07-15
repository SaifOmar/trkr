package types

import (
	"time"
)

const (
	START  = "start"
	END    = "end"
	PAUSE  = "pause"
	RESUME = "resume"
)

type EventType string

type Process struct {
	Name      string
	Pid       int
	Ppid      int
	Tgid      int
	StartTime time.Time
	IsParent  bool
}

type Event struct {
	Type    EventType // start, end, pause, resume
	Process *Process
	Time    time.Time
}

type SessionEvent struct {
	Session   *Session
	EventType // start, end, pause, resume
}

type Session struct {
	StartTime time.Time
	EndTime   time.Time
	Duration  time.Duration

	Proc *Process
}
