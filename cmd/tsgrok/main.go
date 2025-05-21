package main

import (
	"fmt"
	"os"
	"sync"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/joho/godotenv"
	"github.com/jonson/tsgrok/internal/funnel"
	"github.com/jonson/tsgrok/internal/tui"
	"github.com/jonson/tsgrok/internal/util"
)

func main() {
	serverErrorLog := util.NewServerErrorLog()

	err := godotenv.Load()
	if err != nil {
		serverErrorLog.Println("Error loading .env file")
	}

	if util.GetAuthKey() == "" {
		fmt.Printf("Missing env var %s, please set it and try again.\n", util.AuthKeyEnvVar)
		os.Exit(1)
	}

	messageBus := &util.MessageBusImpl{}
	funnelRegistry := funnel.NewFunnelRegistry()
	httpServer, err := funnel.NewHttpServer(util.GetProxyHttpPort(), messageBus, funnelRegistry, serverErrorLog)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating HTTP server: %v\n", err)
		os.Exit(1)
	}

	m := tui.InitialModel(funnelRegistry, serverErrorLog)

	go func() {
		err := httpServer.Start()
		if err != nil {
			// channel to send error to main thread
			fmt.Fprintf(os.Stderr, "Error starting HTTP server: %v\n", err)
		}
	}()

	cleanup := func() {
		if len(funnelRegistry.Funnels) == 0 {
			return
		}

		if len(funnelRegistry.Funnels) == 1 {
			fmt.Printf("Closing tunnel\n")
		} else {
			fmt.Printf("Closing %d tunnels\n", len(funnelRegistry.Funnels))
		}

		var wg sync.WaitGroup
		for _, f := range funnelRegistry.Funnels {
			wg.Add(1)
			go func(f funnel.Funnel) {
				defer wg.Done()
				err := f.Destroy()
				// should we log it?
				if err != nil {
					fmt.Printf("Error destroying tunnel: %v\n", err)
				}
			}(f)
		}
		wg.Wait()
	}
	defer cleanup()

	p := tea.NewProgram(m, tea.WithAltScreen())

	messageBus.SetProgram(p)

	if _, err := p.Run(); err != nil {
		fmt.Println(err)
	}

}
