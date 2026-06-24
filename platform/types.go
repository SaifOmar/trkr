package platform

import "time"

type Process struct {
	Name      string    `json:"name"`
	Pid       int       `json:"pid"`
	Ppid      int       `json:"ppid"`
	Tgid      int       `json:"tgid"`
	StartTime time.Time `json:"start_time"`
}
