package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/SaifOmar/trkr/server"
	"github.com/SaifOmar/trkr/store"
	"github.com/SaifOmar/trkr/trkr"
	"github.com/SaifOmar/trkr/types"
	"github.com/subosito/gotenv"
)

func main() {
	if err := gotenv.Load(); err != nil {
		panic(fmt.Sprintf("Error loading .env file: %v", err))
	}

	ctx, cancel := context.WithCancel(context.Background())
	terminate := make(chan os.Signal, 1)
	signal.Notify(terminate,
		syscall.SIGTERM,
		syscall.SIGINT,
	)
	errChan := make(chan error)
	var activeSessions []*types.Session

	localStore := store.New(store.Config{}, ctx)

	processesChan := make(chan []*types.Process)
	ticker := time.NewTicker(time.Second * 2)
	t := trkr.New(ctx, processesChan, ticker, localStore)
	done := make(chan struct{})
	server := server.New(
		t,
		localStore,
		os.Getenv("SUPABASE_URL"),
		os.Getenv("SUPABASE_PUBLIC_KEY"),
		os.Getenv("SUPABASE_PRIVATE_KEY"),
		":45098")

	removeSession := func(sessions []*types.Session, session *types.Session) []*types.Session {
		for i, s := range sessions {
			if s.ID == session.ID {
				return append(sessions[:i], sessions[i+1:]...)
			}
		}
		return sessions
	}

	go func() {
		err := server.Start()
		if err != nil {
			errChan <- err
		}
	}()

	go func() {
		t.Run()
		close(terminate)
		done <- struct{}{}
	}()

	for {
		select {
		case e := <-t.EventChan:
			switch e.Type {
			case types.START:
				session := &types.Session{
					StartTime: e.Time,
					ProcessID: e.Process.ID,
					Proc:      e.Process,
				}
				activeSessions = append(activeSessions, session)
				fmt.Println(activeSessions)
				server.ActiveSessions = append([]*types.Session(nil), activeSessions...)
				fmt.Println(server.ActiveSessions)
				localStore.CreateProcess(e.Process)
				localStore.CreateSession(session)
			case types.END:
				localStore.UpdateSession(localStore.GetSession(e.Process.ID))
				ses := localStore.GetSession(e.Process.ID)
				ses.Duration = e.Time.Sub(ses.StartTime)
				ses.EndTime = &e.Time
				activeSessions = removeSession(activeSessions, ses)
				fmt.Println(activeSessions)
				server.ActiveSessions = append([]*types.Session(nil), activeSessions...)
				fmt.Println(server.ActiveSessions)
				localStore.UpdateSession(ses)
			}

		case snapshot := <-t.ProcessesChan:
			// TODO : consume for the server
			server.ActiveProcesses = snapshot

		case e := <-errChan:
			fmt.Println("server error: " + e.Error())
			cancel()
		case <-terminate:
			fmt.Println("Shutting down...")
			cancel()
		}

	}
}
