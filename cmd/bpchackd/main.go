package main

import (
	"fmt"
	"os"

	log "github.com/sirupsen/logrus"
)

func run() error {
	log.Info("Starting BPC Hack proxy")

	return nil
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "%v", err)
		os.Exit(1)
	}
}
