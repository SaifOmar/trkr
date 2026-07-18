package server

import (
	"encoding/json"
	"errors"
	"fmt"
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
		fmt.Println(session)
	}
	json.NewEncoder(w).Encode(sessions)

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
