package types

import (
	"gorm.io/gorm"
	"time"
)

const (
	START   = "start"
	RUNNING = "running"
	END     = "end"
	PAUSE   = "pause"
	RESUME  = "resume"
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
	Status    int           `json:"status"`
}

type Process struct {
	BaseModel
	OS         string        `json:"os"`
	DeviceName string        `json:"device_name"`
	Name       string        `json:"name"`
	Pid        int           `json:"pid"`
	Ppid       int           `json:"ppid"`
	StartTime  time.Time     `json:"start_time"`
	IsParent   bool          `json:"is_parent"`
	Duration   time.Duration `json:"duration"`
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

type HyprlandActiveWindow struct {
	Address   string `json:"address"`
	Mapped    bool   `json:"mapped"`
	Hidden    bool   `json:"hidden"`
	At        [2]int `json:"at"`
	Size      [2]int `json:"size"`
	Workspace struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	} `json:"workspace"`
	Monitor         int      `json:"monitor"`
	MonitorID       int      `json:"monitorID"`
	Class           string   `json:"class"`
	Title           string   `json:"title"`
	InitialClass    string   `json:"initialClass"`
	InitialTitle    string   `json:"initialTitle"`
	Pid             int      `json:"pid"`
	Xwayland        bool     `json:"xwayland"`
	Pinned          bool     `json:"pinned"`
	Fullscreen      int      `json:"fullscreen"`
	FullscreenMode  int      `json:"fullscreenMode"`
	FakeFullscreen  bool     `json:"fakeFullscreen"`
	Group           []string `json:"group"`
	Tags            []string `json:"tags"`
	Swallowing      string   `json:"swallowing"`
	FocusHistoryID  int      `json:"focusHistoryID"`
	InhibitingIdle  bool     `json:"inhibitingIdle"`
	ForeignToplevel struct {
		// fields depend on Hyprland version
	} `json:"foreign_toplevel"`
	GroupedBy []string `json:"groupedBy"`
}
