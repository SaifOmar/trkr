package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"slices"
	"strings"
	"syscall"
	"time"

	"github.com/SaifOmar/trkr/server"
	"github.com/SaifOmar/trkr/store"
	"github.com/SaifOmar/trkr/trkr"
	"github.com/SaifOmar/trkr/types"
	"github.com/subosito/gotenv"
)

type flagSlise []string

func (s *flagSlise) String() string {
	return strings.Join(*s, ",")
}

func (s *flagSlise) Set(value string) error {
	for name := range strings.SplitSeq(value, ",") {
		name = strings.TrimSpace(name)
		if name != "" {
			*s = append(*s, name)
		}
	}
	return nil
}

func main() {
	// .env is optional — env vars and flags work without it
	_ = gotenv.Load()

	SUPABASE_URL := flag.String("sb_url", os.Getenv("SUPABASE_URL"), "supabase url")
	SUPABASE_PUBLIC_KEY := flag.String("sb_pub_key", os.Getenv("SUPABASE_PUBLIC_KEY"), "supabase public key")
	SUPABASE_PRIVATE_KEY := flag.String("sb_priv_key", os.Getenv("SUPABASE_PRIVATE_KEY"), "supabase private key")
	PORT := flag.String("port", os.Getenv("PORT"), "port to listen on")
	var WATCH flagSlise
	flag.Var(&WATCH, "watch", "comma separated list of processes to watch")

	flag.Parse()

	if *PORT == "" {
		*PORT = "45098"
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
	if len(WATCH) > 0 {
		for _, name := range WATCH {
			exists := localStore.GetAutoWatch(name)
			if exists.Name == name {
				continue
			}
			localStore.CreateAutoWatch(&types.AutoWatch{Name: name})
		}
	}

	processesChan := make(chan []*types.Process, 10)
	ticker := time.NewTicker(time.Second * 2)
	t := trkr.New(ctx, processesChan, ticker, localStore)
	done := make(chan struct{})
	server := server.New(
		t,
		localStore,
		*SUPABASE_URL,
		*SUPABASE_PUBLIC_KEY,
		*SUPABASE_PRIVATE_KEY,
		*PORT)

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
	fmt.Println("Server is running at http://localhost" + server.Server.Addr)

	go func() {
		t.Run()
		done <- struct{}{}
	}()

loop:
	for {
		select {
		case e := <-t.EventChan:
			switch e.Type {
			case types.START:
				e.Process.Duration = time.Since(e.Process.StartTime)
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
				pid := e.Process.Pid
				idx := slices.IndexFunc(server.ActiveSessions, func(s *types.Session) bool {
					return s.Proc.Pid == pid
				})
				var ses *types.Session
				if idx != -1 {
					ses = server.ActiveSessions[idx]
					// ses := localStore.GetSessionByProcessID(e.Process.Pid)
					fmt.Println("ses: ", ses)
					ses.Duration = e.Time.Sub(ses.StartTime)
					ses.EndTime = &e.Time
					activeSessions = removeSession(activeSessions, ses)
					fmt.Println(activeSessions)
					server.ActiveSessions = append([]*types.Session(nil), activeSessions...)
					fmt.Println(server.ActiveSessions)
					localStore.UpdateSession(ses)
					proc := localStore.GetProcess(ses.ProcessID)
					if proc != nil {
						proc.Duration = e.Time.Sub(proc.StartTime)
						localStore.UpdateProcess(proc)
					}
				}
			}

		case snapshot := <-t.ProcessesChan:
			// TODO : consume for the server
			server.ActiveProcesses = snapshot

		case e := <-errChan:
			fmt.Println("server error: " + e.Error())
			cancel()
			break loop
		case <-terminate:
			debug(t)
			fmt.Println("Shutting down...")
			cancel()
			break loop
		}
	}
	<-done
	e := time.Now()
	for _, ses := range server.ActiveSessions {
		ses.EndTime = &e
		ses.Duration = e.Sub(ses.StartTime)
		localStore.UpdateSession(ses)
		proc := localStore.GetProcess(ses.ProcessID)
		if proc != nil {
			proc.Duration = e.Sub(proc.StartTime)
			localStore.UpdateProcess(proc)
		}
	}

	debug(t)
	fmt.Println("Bye")
	os.Exit(0)
}

func debug(t *trkr.Traker) {
	log.Printf("=== Tracker Debug ===\n"+
		"Processes:      %d tracked\n"+
		"WatchList:      %+v\n"+
		"AutoWatchList:  %v\n"+
		"EventChan:      len=%d cap=%d\n"+
		"ProcessesChan:  len=%d cap=%d\n"+
		"Ctx:            %v\n"+
		"Ticker running: %v\n",
		len(*t.Procceess),
		t.WatchList,
		t.AutoWatchList,
		len(t.EventChan), cap(t.EventChan),
		len(t.ProcessesChan), cap(t.ProcessesChan),
		t.Ctx,
		t.Ticker != nil,
	)
}
