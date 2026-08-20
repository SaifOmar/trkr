package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
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

type installPaths struct {
	binDir     string
	configDir  string
	configFile string
	binary     string
}

func getInstallPaths() (installPaths, error) {
	OS := runtime.GOOS
	home, err := os.UserHomeDir()
	if err != nil {
		return installPaths{}, fmt.Errorf("get home dir: %w", err)
	}
	var p installPaths
	switch OS {
	case "linux":
		p.binDir = filepath.Join(home, ".local", "bin")
		p.configDir = filepath.Join(home, ".config", "systemd", "user")
		p.configFile = filepath.Join(p.configDir, "trkr.service")
		p.binary = filepath.Join(p.binDir, "trkr")
	case "windows":
		p.binDir = filepath.Join(home, "AppData", "Local", "trkr")
		p.configDir = p.binDir
		p.binary = filepath.Join(p.binDir, "trkr.exe")
	}
	return p, nil
}

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

func handleInstall() error {
	OS := runtime.GOOS

	paths, err := getInstallPaths()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(paths.binDir, 0o755); err != nil {
		return fmt.Errorf("create bin dir: %w", err)
	}
	if err := os.MkdirAll(paths.configDir, 0o755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	src, err := os.Executable()
	if err != nil {
		return fmt.Errorf("get executable path: %w", err)
	}
	if err := copyFile(src, paths.binary, 0o755); err != nil {
		return fmt.Errorf("copy binary: %w", err)
	}

	switch OS {
	case "linux":
		unit := fmt.Sprintf(`[Unit]
Description=trkr process tracking daemon
After=graphical-session.target

[Service]
ExecStart=%s
Restart=on-failure

[Install]
WantedBy=default.target
`, paths.binary)

		if err := os.WriteFile(paths.configFile, []byte(unit), 0o644); err != nil {
			return fmt.Errorf("write unit file: %w", err)
		}

		if err := runCmd("systemctl", "--user", "daemon-reload"); err != nil {
			return err
		}
		if err := runCmd("systemctl", "--user", "enable", "--now", "trkr.service"); err != nil {
			return err
		}

	case "windows":
		if err := runCmd("schtasks", "/create", "/tn", "trkr", "/tr", paths.binary, "/sc", "onlogon", "/rl", "limited", "/f"); err != nil {
			return fmt.Errorf("create scheduled task: %w", err)
		}
		if err := runCmd("schtasks", "/run", "/tn", "trkr"); err != nil {
			return fmt.Errorf("start scheduled task: %w", err)
		}
	}

	fmt.Printf("trkr installed to %s\n", paths.binary)
	return nil
}

func handleUninstall() error {
	OS := runtime.GOOS

	paths, err := getInstallPaths()
	if err != nil {
		return err
	}

	switch OS {
	case "linux":
		runCmd("systemctl", "--user", "stop", "trkr.service")
		runCmd("systemctl", "--user", "disable", "trkr.service")
		os.Remove(paths.configFile)
		runCmd("systemctl", "--user", "daemon-reload")

	case "windows":
		runCmd("schtasks", "/end", "/tn", "trkr")
		runCmd("schtasks", "/delete", "/tn", "trkr", "/f")
	}

	os.Remove(paths.binary)

	fmt.Println("trkr uninstalled")
	return nil
}

func copyFile(src, dst string, perm os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, perm)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}

func runCmd(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func main() {
	_ = gotenv.Load()

	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "install":
			if err := handleInstall(); err != nil {
				log.Fatal(err)
			}
			return
		case "uninstall":
			if err := handleUninstall(); err != nil {
				log.Fatal(err)
			}
			return
		}
	}

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

	getTargetSession := func(pid int, sessions []*types.Session) *types.Session {
		idx := slices.IndexFunc(sessions, func(s *types.Session) bool {
			return s.Proc.Pid == pid
		})
		var ses *types.Session
		if idx != -1 {
			ses = sessions[idx]
			return ses
		}
		return nil
	}
	wtl := func(e types.Event) {
		f, err := os.OpenFile(
			"logevents.txt",
			os.O_CREATE|os.O_WRONLY|os.O_APPEND,
			0644,
		)
		if err != nil {
			log.Fatal(err)
		}
		defer f.Close()

		_, err = fmt.Fprintf(f, "eventType: %s, pid: %d, name: %s\n", e.Type, e.Process.Pid, e.Process.Name)
		if err != nil {
			log.Fatal(err)
		}
	}
loop:
	for {
		select {
		case e := <-t.EventChan:
			wtl(e)
			switch e.Type {
			case types.START:
				e.Process.Duration = time.Since(e.Process.StartTime)
				session := &types.Session{
					StartTime: e.Time,
					ProcessID: e.Process.ID,
					Proc:      e.Process,
					Status:    0,
				}
				activeSessions = append(activeSessions, session)
				server.ActiveSessions = append([]*types.Session(nil), activeSessions...)
				localStore.CreateProcess(e.Process)
				localStore.CreateSession(session)
			case types.PAUSE:
				ses := getTargetSession(e.Process.Pid, activeSessions)
				if ses != nil {
					ses.Duration = e.Time.Sub(ses.StartTime)
					ses.EndTime = &e.Time
					ses.Status = 1
					serverSession := getTargetSession(e.Process.Pid, server.ActiveSessions)
					if serverSession != nil {
						server.ActiveSessions = removeSession(activeSessions, serverSession)
					}
				}
			case types.RESUME:
				ses := getTargetSession(e.Process.Pid, activeSessions)
				fmt.Printf("Resume %d %s\n", e.Process.Pid, e.Process.Name)
				fmt.Printf("ActiveSessions: %v\n", activeSessions)

				if ses != nil {
					ses.StartTime = e.Time // resuming — this is effectively a new segment
					ses.EndTime = nil
					ses.Status = 0
					serverSession := getTargetSession(e.Process.Pid, server.ActiveSessions)
					if serverSession != nil {
						server.ActiveSessions = removeSession(server.ActiveSessions, serverSession)
					}
					server.ActiveSessions = append(server.ActiveSessions, ses)
				}
			case types.END:
				ses := getTargetSession(e.Process.Pid, activeSessions)
				// pid := e.Process.Pid
				// idx := slices.IndexFunc(server.ActiveSessions, func(s *types.Session) bool {
				// 	return s.Proc.Pid == pid
				// })
				// var ses *types.Session
				if ses != nil {
					// ses = server.ActiveSessions[idx]
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
					} else {
						// TODO : Handle this case
						// TODO : LOG mayeb ?
					}

				} else {
					// TODO : Handle this case
					// TODO : LOG mayeb ?
				}
			}

		case snapshot := <-t.ProcessesChan:
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
		t.Watched,
		t.AutoWatchList,
		len(t.EventChan), cap(t.EventChan),
		len(t.ProcessesChan), cap(t.ProcessesChan),
		t.Ctx,
		t.Ticker != nil,
	)
}
