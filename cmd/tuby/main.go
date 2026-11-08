package main

import (
	"fmt"
	"os"

	"git.nexacloud.dev/MrSnakeDoc/tuby/internal"
)

func main() {
	app, err := internal.New()
	if err != nil {
		fmt.Fprintf(os.Stderr, "startup failed: %v\n", err)
		os.Exit(1)
	}
	if err := app.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "application stopped with error: %v\n", err)
		os.Exit(1)
	}
}
