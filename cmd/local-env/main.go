package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"

	"github.com/fifi/internal/localdev"
)

func main() {
	flags := flag.NewFlagSet("local-env", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	test := flags.Bool("test", false, "use isolated test database and Redis URLs")
	flags.Usage = func() { fmt.Fprintln(os.Stderr, "usage: local-env [--test] COMMAND [ARGS...]") }
	if err := flags.Parse(os.Args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return
		}
		os.Exit(2)
	}
	args := flags.Args()
	if len(args) == 0 {
		flags.Usage()
		os.Exit(2)
	}
	environment, err := localdev.LoadEnvironment(os.Environ(), ".env", *test)
	if err != nil {
		fail(err.Error())
	}
	// Command lookup must use the same PATH that will be exported to the command.
	for _, entry := range environment {
		key, value, _ := strings.Cut(entry, "=")
		if err := os.Setenv(key, value); err != nil {
			fail("cannot export environment")
		}
	}
	command, err := exec.LookPath(args[0])
	if err != nil {
		fail("cannot locate command")
	}
	if err := syscall.Exec(command, args, environment); err != nil {
		fail("cannot execute command")
	}
}

func fail(message string) {
	fmt.Fprintln(os.Stderr, "local-env:", message)
	os.Exit(1)
}
