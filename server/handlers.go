package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/SaifOmar/trkr/types"
	"gorm.io/gorm"
)

func (s *Server) GetAllProcesses(w http.ResponseWriter, r *http.Request) {
	processes := s.store.GetAllProcess()
	json.NewEncoder(w).Encode(processes)
}

func (s *Server) QueryActiveProcesses(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	var processes []*types.Process

	switch {
	case q.Has("pid"):
		pid, err := strconv.Atoi(q.Get("pid"))
		if err != nil {
			http.Error(w, "invalid pid", http.StatusBadRequest)
			return
		}
		for _, p := range s.ActiveProcesses {
			if p.Pid == pid {
				processes = append(processes, p)
			}
		}

	case q.Has("ppid"):
		ppid, err := strconv.Atoi(q.Get("ppid"))
		if err != nil {
			http.Error(w, "invalid ppid", http.StatusBadRequest)
			return
		}
		for _, p := range s.ActiveProcesses {
			if p.Ppid == ppid {
				processes = append(processes, p)
			}
		}

	case q.Has("device_name"):
		name := q.Get("device_name")
		for _, p := range s.ActiveProcesses {
			if p.DeviceName == name {
				processes = append(processes, p)
			}
		}

	default:
		http.Error(w, "must provide pid, ppid, or device_name", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(processes); err != nil {
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
	}
}
func (s *Server) QueryProcesses(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	var processes []types.Process

	switch {
	case q.Has("pid"):
		pid, err := strconv.Atoi(q.Get("pid"))
		if err != nil {
			http.Error(w, "invalid pid", http.StatusBadRequest)
			return
		}
		if err := s.store.DB.Where("pid = ?", pid).Find(&processes).Error; err != nil {
			http.Error(w, "query failed", http.StatusInternalServerError)
			return
		}

	case q.Has("ppid"):
		ppid, err := strconv.Atoi(q.Get("ppid"))
		if err != nil {
			http.Error(w, "invalid ppid", http.StatusBadRequest)
			return
		}
		if err := s.store.DB.Where("ppid = ?", ppid).Find(&processes).Error; err != nil {
			http.Error(w, "query failed", http.StatusInternalServerError)
			return
		}

	case q.Has("device_name"):
		name := q.Get("device_name")
		if err := s.store.DB.Where("device_name = ?", name).Find(&processes).Error; err != nil {
			http.Error(w, "query failed", http.StatusInternalServerError)
			return
		}

	default:
		http.Error(w, "must provide pid, ppid, or device_name", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(processes); err != nil {
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
	}
}

func (s *Server) GetProcess(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	strings.TrimSpace(id)
	i, err := strconv.ParseUint(id, 10, 64)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	process := s.store.GetProcess(uint(i))
	json.NewEncoder(w).Encode(process)
}
func (s *Server) GetActiveProcesses(w http.ResponseWriter, r *http.Request) {
	processes := s.ActiveProcesses
	json.NewEncoder(w).Encode(processes)

}
func (s *Server) GetAutoWatch(w http.ResponseWriter, r *http.Request) {
	autoWatch := s.store.GetAllAutoWatch()
	json.NewEncoder(w).Encode(autoWatch)
}

func (s *Server) RemoveAutoWatch(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	strings.TrimSpace(name)
	var autoWatch types.AutoWatch
	err := s.store.DB.Where("name = ?", name).First(&autoWatch).Error
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	s.store.DeleteAutoWatch(autoWatch.ID)
	s.tr.RemoveAutoWatch(autoWatch.Name)
	w.WriteHeader(http.StatusOK)
}

func (s *Server) CreateAutoWatch(w http.ResponseWriter, r *http.Request) {
	var autoWatch types.AutoWatch
	if err := json.NewDecoder(r.Body).Decode(&autoWatch); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	var existing types.AutoWatch
	err := s.store.DB.Where("name = ?", autoWatch.Name).First(&existing).Error

	if err == nil {
		// found a row -> name already taken
		http.Error(w, "autowatch with this name already exists", http.StatusConflict)
		return
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		// some other real DB error (connection, etc.)
		http.Error(w, "failed to check existing autowatch", http.StatusInternalServerError)
		return
	}
	// err is ErrRecordNotFound here -> name is free, fall through and create
	s.store.CreateAutoWatch(&autoWatch)

	s.tr.AddAutoWatch(autoWatch.Name)
	w.WriteHeader(http.StatusCreated)
}

func (s *Server) GetActiveSessions(w http.ResponseWriter, r *http.Request) {
	// TODO : get current session from the main process
	sessions := s.ActiveSessions
	for _, session := range sessions {
		fmt.Println("server session: ", session)
	}
	json.NewEncoder(w).Encode(sessions)
}

func (s *Server) StopActiveSession(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Pid  int    `json:"pid"`
		Ppid int    `json:"ppid"`
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	// Try PID first (most specific)
	if req.Pid > 1 {
		fmt.Println(req.Pid, req.Ppid, req.Name)
		s.tr.StopWatching(req.Pid)
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "stopped", "count": "1"})
		return
	}

	// Try PPID + Name
	// if req.Ppid > 0 && req.Name != "" {
	// 	procs := s.tr.StopWatchingByName(req.Name)
	// 	for _, p := range procs {
	// 		if p.Ppid == req.Ppid {
	// 			s.tr.StopWatching(p.Pid)
	// 		}
	// 	}
	// 	now := time.Now()
	// 	var remaining []*types.Session
	// 	stopped := 0
	// 	for _, ses := range s.ActiveSessions {
	// 		if ses.Proc != nil && ses.Proc.Ppid == req.Ppid && strings.EqualFold(ses.Proc.Name, req.Name) {
	// 			ses.Duration = now.Sub(ses.StartTime)
	// 			ses.EndTime = &now
	// 			s.store.UpdateSession(ses)
	// 			stopped++
	// 		} else {
	// 			remaining = append(remaining, ses)
	// 		}
	// 	}
	// 	s.ActiveSessions = remaining
	// 	if stopped > 0 {
	// 		w.WriteHeader(http.StatusOK)
	// 		json.NewEncoder(w).Encode(map[string]string{"status": "stopped", "count": strconv.Itoa(stopped)})
	// 		return
	// 	}
	// }

	// Fallback: stop by name only
	// if req.Name != "" {
	// 	procs := s.tr.StopWatchingByName(req.Name)
	// 	for _, p := range procs {
	// 		s.tr.StopWatching(p.Pid)
	// 	}
	// 	now := time.Now()
	// 	var remaining []*types.Session
	// 	stopped := 0
	// 	for _, ses := range s.ActiveSessions {
	// 		if ses.Proc != nil && strings.EqualFold(ses.Proc.Name, req.Name) {
	// 			ses.Duration = now.Sub(ses.StartTime)
	// 			ses.EndTime = &now
	// 			s.store.UpdateSession(ses)
	// 			stopped++
	// 		} else {
	// 			remaining = append(remaining, ses)
	// 		}
	// 	}
	// 	s.ActiveSessions = remaining
	// 	if stopped > 0 {
	// 		w.WriteHeader(http.StatusOK)
	// 		json.NewEncoder(w).Encode(map[string]string{"status": "stopped", "count": strconv.Itoa(stopped)})
	// 		return
	// 	}
	// }

	http.Error(w, "no active session found", http.StatusNotFound)
}

func (s *Server) StartManualWatch(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Pid         int  `json:"pid"`
		WatchParent bool `json:"watch_parent"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.Pid <= 0 {
		http.Error(w, "pid is required", http.StatusBadRequest)
		return
	}

	targetPid := req.Pid
	if req.WatchParent {
		// Find child process, then find its parent
		var child *types.Process
		for _, p := range s.ActiveProcesses {
			if p.Pid == req.Pid {
				child = p
				break
			}
		}
		if child == nil {
			http.Error(w, "process not found", http.StatusNotFound)
			return
		}

		log.Printf("[StartManualWatch] child: pid=%d ppid=%d name=%q is_parent=%v", child.Pid, child.Ppid, child.Name, child.IsParent)
		log.Printf("[StartManualWatch] all same-name processes for %q:", child.Name)
		for _, p := range s.ActiveProcesses {
			if strings.EqualFold(p.Name, child.Name) {
				log.Printf("  pid=%d ppid=%d name=%q is_parent=%v", p.Pid, p.Ppid, p.Name, p.IsParent)
			}
		}

		found := false
		for _, p := range s.ActiveProcesses {
			if strings.EqualFold(p.Name, child.Name) && p.IsParent {
				log.Printf("[StartManualWatch] chosen parent: pid=%d ppid=%d name=%q is_parent=%v", p.Pid, p.Ppid, p.Name, p.IsParent)
				targetPid = p.Pid
				found = true
				break
			}
		}
		if !found {
			log.Printf("[StartManualWatch] no parent found for %q", child.Name)
			http.Error(w, "parent process not found", http.StatusNotFound)
			return
		}
	}

	if s.tr.AddManualWatch(targetPid) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "watching"})
	} else {
		http.Error(w, "process not found or already watched", http.StatusNotFound)
	}
}

func (s *Server) GetAllSessions(w http.ResponseWriter, r *http.Request) {
	sessions := s.store.GetAllSession()
	json.NewEncoder(w).Encode(sessions)
}

func (s *Server) GetSession(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	strings.TrimSpace(id)
	i, err := strconv.ParseUint(id, 10, 64)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	session := s.store.GetSession(uint(i))
	json.NewEncoder(w).Encode(session)
}
