package main

import (
	"context"
	"fmt"
	// "log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/SaifOmar/trkr/platform"
	"github.com/SaifOmar/trkr/trkr"
	"github.com/SaifOmar/trkr/types"
	_ "modernc.org/sqlite"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	processesChan := make(chan []*types.Process)
	ticker := time.NewTicker(time.Second * 2)
	terminate := make(chan os.Signal, 1)

	signal.Notify(terminate,
		syscall.SIGTERM,
		syscall.SIGINT,
	)

	t := trkr.New(ctx, processesChan, ticker)
	// TODO : next
	// store := store.New()

	platform.PollProc(t.Procceess)

	go func() {
		t.Run()
		close(terminate)
	}()

	for {
		select {
		case e := <-t.EventChan:
			switch e.Type {
			case types.START:
				fmt.Println("starting: ", e.Process.Name)
				fmt.Println("starting: ", e.Process.Name)
				writeToTestFile(e.Process)
			case types.END:
				fmt.Println("ending: ", e.Process.Name)
				fmt.Println("ending: ", e.Process.Name)
				t.Save(e.Process)
				writeToTestFile(e.Process)
			}
		case snapshot := <-t.ProcessesChan:
			// TODO : consume for the server
			fmt.Println("snapshot: ", snapshot)
		//

		case <-terminate:
			fmt.Println("Shutting down...")
			cancel()
			os.Exit(1)
		}

	}
}

func writeToTestFile(p *types.Process) {
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Println(err)
		return
	}

	fileName := filepath.Join(cwd, "test.txt")

	f, err := os.OpenFile(fileName, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer f.Close()

	if _, err := fmt.Fprintf(f, "%+v\n", p); err != nil {
		fmt.Println(err)
	}
}
