package main

import (
	"context"
	"fmt"
	"os"
	"strconv"

	"github.com/jin-take/SplitAgents/internal/app"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run() error {
	a, err := app.New()
	if err != nil {
		return err
	}
	args := os.Args[1:]
	if len(args) < 2 {
		return usage()
	}
	switch args[0] + " " + args[1] {
	case "run start":
		return a.Start(context.Background())
	case "room open":
		if len(args) < 3 {
			return usage()
		}
		return a.OpenRoom(context.Background(), args[2])
	case "pane run":
		room := ""
		pane := 1
		for i := 2; i < len(args); i++ {
			if args[i] == "--room" && i+1 < len(args) {
				room = args[i+1]
				i++
			} else if args[i] == "--pane" && i+1 < len(args) {
				pane, _ = strconv.Atoi(args[i+1])
				i++
			}
		}
		if room == "" {
			return fmt.Errorf("--room is required")
		}
		return a.RunPane(context.Background(), room, pane)
	default:
		return usage()
	}
}

func usage() error {
	fmt.Println(`SplitAgents

Usage:
  chatgpt run start
  chatgpt room open <room-id>
  chatgpt pane run --room <room-id> --pane <1..3>`)
	return nil
}
