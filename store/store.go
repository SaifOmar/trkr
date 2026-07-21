package store

import (
	"context"
	"fmt"

	"github.com/SaifOmar/trkr/types"
	"github.com/glebarez/sqlite" // pure Go, no cgo
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type DBType int

const (
	SQLITE = iota
	POSTGRES
	DOCKER
)

type Config struct {
	ConnctionType DBType
	Port          string
	Host          string
	User          string
	Password      string
	Database      string
}

type Store struct {
	ctx context.Context
	DB  *gorm.DB
	Config
}

func New(c Config, ctx context.Context) *Store {
	s := &Store{Config: c, ctx: ctx}
	s.init()
	return s
}

func (s *Store) init() {
	switch s.ConnctionType {
	case POSTGRES:
		db, err := gorm.Open(postgres.Open(fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable", s.Host, s.Port, s.User, s.Password, "postgres")), &gorm.Config{})
		if err != nil {
			panic(fmt.Sprintf("failed to connect to postgres database: %v", err))
		}
		db.AutoMigrate(&types.Process{}, &types.Session{}, &types.AutoWatch{})
		s.DB = db
	default:
		db, err := gorm.Open(sqlite.Open("test.db"))
		if err != nil {
			panic(fmt.Sprintf("failed to connect to sqlite database: %v", err))
		}
		db.AutoMigrate(&types.Process{}, &types.Session{}, &types.AutoWatch{})
		s.DB = db
	}
}

func (s *Store) CreateAutoWatch(autoWatch *types.AutoWatch) {
	s.DB.Create(autoWatch)
}

func (s *Store) GetAllAutoWatch() []*types.AutoWatch {
	var autoWatch []*types.AutoWatch
	s.DB.Find(&autoWatch)
	return autoWatch
}

func (s *Store) CreateProcess(process *types.Process) {
	s.DB.Create(process)
}

func (s *Store) CreateSession(session *types.Session) {
	s.DB.Create(session)
}

func (s *Store) GetProcess(id uint) *types.Process {
	var process types.Process
	s.DB.Where("id = ?", id).First(&process)
	return &process
}
func (s *Store) GetSession(id uint) *types.Session {
	var session types.Session
	s.DB.Preload("Proc").Where("id = ?", id).First(&session)
	return &session
}

func (s *Store) GetSessions(processID uint) []*types.Session {
	var sessions []*types.Session
	s.DB.Where("process_id = ?", processID).Find(&sessions)
	return sessions
}
func (s *Store) GetProcesses() []*types.Process {
	var processes []*types.Process
	s.DB.Find(&processes)
	return processes
}
func (s *Store) UpdateProcess(process *types.Process) {
	s.DB.Save(process)
}

func (s *Store) UpdateSession(session *types.Session) {
	s.DB.Save(session)
}
func (s *Store) DeleteProcess(id uint) {
	s.DB.Delete(&types.Process{}, id)
}

func (s *Store) GetAllSession() []*types.Session {
	var sessions []*types.Session
	s.DB.Preload("Proc").Find(&sessions)
	return sessions
}

func (s *Store) GetAllProcess() []*types.Process {
	var processes []*types.Process
	s.DB.Find(&processes)
	return processes
}

func (s *Store) GetAutoWatch(qr any) *types.AutoWatch {
	switch v := qr.(type) {
	case uint:
		var autoWatch types.AutoWatch
		s.DB.Where("id = ?", v).First(&autoWatch)
		return &autoWatch
	case string:
		var autoWatch types.AutoWatch
		s.DB.Where("name = ?", v).First(&autoWatch)
		return &autoWatch
	}
	return nil
}

func (s *Store) DeleteAutoWatch(id uint) {
	s.DB.Unscoped().Delete(&types.AutoWatch{}, id)
}
