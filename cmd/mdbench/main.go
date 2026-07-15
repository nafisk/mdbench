package main

import (
	"errors"
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"
	"github.com/nafiskhan/mdbench/internal/app"
	"github.com/nafiskhan/mdbench/internal/tui"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	config, err := app.LoadConfig(args)
	if errors.Is(err, app.ErrHelp) {
		fmt.Println(app.Usage())
		return 0
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "mdbench:", err)
		fmt.Fprintln(os.Stderr, app.Usage())
		return 2
	}
	service, err := app.NewService(config)
	if err != nil {
		fmt.Fprintln(os.Stderr, "mdbench:", err)
		return 1
	}
	if _, err := tea.NewProgram(tui.New(service, config)).Run(); err != nil {
		fmt.Fprintln(os.Stderr, "mdbench:", err)
		return 1
	}
	return 0
}
