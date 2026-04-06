package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	notify "github.com/b0bbywan/go-odio-notify"
)

var version = "dev"

func main() {
	flag.Usage = usage
	pulseServer := flag.String("pulse-server", "", "PulseAudio server address")
	soundsDir := flag.String("sounds-dir", "", "custom sounds directory")
	timeout := flag.Duration("timeout", 5*time.Second, "connection timeout")

	flag.Parse()

	args := flag.Args()
	if len(args) < 1 {
		usage()
		os.Exit(1)
	}

	switch args[0] {
	case "play":
		if len(args) < 2 {
			fatal("usage: odio-notify play <event> [event...]")
		}
		play(args[1:], *pulseServer, *soundsDir, *timeout)
	case "list":
		for _, e := range notify.AllEvents {
			fmt.Println(e)
		}
	case "version":
		fmt.Printf("odio-notify %s\n", version)
	default:
		fatal("unknown command: %s", args[0])
	}
}

func play(events []string, pulseServer, soundsDir string, timeout time.Duration) {
	n, err := notify.New(notify.Config{
		Backend:        "pulse",
		PulseServer:    pulseServer,
		SoundsDir:      soundsDir,
		ConnectTimeout: timeout,
	})
	if err != nil {
		fatal("%v", err)
	}
	defer n.Close()

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	if err := n.WaitReady(ctx); err != nil {
		fatal("%v", err)
	}

	for _, name := range events {
		event := notify.SoundEvent(name)
		if err := n.PlaySync(context.Background(), event); err != nil {
			fatal("play %s: %v", name, err)
		}
	}
}

func fatal(msg string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, msg+"\n", args...)
	os.Exit(1)
}

func usage() {
	fmt.Print(`Usage: odio-notify [options] <command> [args]

Commands:
  play <event> [event...]  play notification sounds
  list                     list available events
  version                  print version

Options:
`)
	flag.PrintDefaults()
}
