package types

import (
	"gorm.io/gorm"
	"time"
)

const (
	START  = "start"
	END    = "end"
	PAUSE  = "pause"
	RESUME = "resume"
)

type BaseModel struct {
	ID        uint           `json:"id" gorm:"primarykey"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`
}

type Session struct {
	BaseModel
	StartTime time.Time     `json:"start_time"`
	EndTime   *time.Time    `json:"end_time"`
	ProcessID uint          `json:"process_id"`
	Duration  time.Duration `json:"duration"`
	Proc      *Process      `json:"proc,omitempty" gorm:"foreignKey:ProcessID;references:ID"`
}

type Process struct {
	BaseModel
	OS         string    `json:"os"`
	DeviceName string    `json:"device_name"`
	Name       string    `json:"name"`
	Pid        int       `json:"pid"`
	Ppid       int       `json:"ppid"`
	Tgid       int       `json:"tgid"`
	StartTime  time.Time `json:"start_time"`
	IsParent   bool      `json:"is_parent"`
}

type AutoWatch struct {
	BaseModel
	Name string `json:"name"`
}
type EventType string

type Event struct {
	Type    EventType // start, end, pause, resume
	Process *Process
	Time    time.Time
}

type SessionEvent struct {
	Session   *Session
	EventType // start, end, pause, resume
}
