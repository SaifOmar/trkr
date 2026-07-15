package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
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
				t.WatchList[e.Process.Pid] = e.Type
				go t.Watch(e.Process)
			case types.END:
				fmt.Println("ending: ", e.Process.Name)
				t.WatchList[e.Process.Pid] = e.Type
				t.Save(e.Process)
			}
		case p := <-t.ProcessesChan:
			fmt.Println("got new procs", p)
		case <-terminate:
			fmt.Println("Shutting down...")
			cancel()
			os.Exit(1)
		}

	}
}
