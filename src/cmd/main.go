package main

import (
	"flag"
	"fmt"
	"os"
	ossignal "os/signal"
	"syscall"
	"time"

	"oom-signal/src/internal/memory_reader"
	"oom-signal/src/internal/monitor"
	"oom-signal/src/internal/signal"
	"oom-signal/src/internal/threshold"
)

func main() {
	th := threshold.Default()
	flag.Var(&th, "threshold", "Memory threshold: fraction 0.0–1.0 or megabytes e.g. 100MB")
	pollInterval := flag.Duration("poll-interval", 100*time.Millisecond, "Cgroup memory poll interval")
	targetPid := flag.Int("pid", 1, "PID to signal when threshold is exceeded")
	sigName := flag.String("signal", "SIGUSR1", "Signal to send (e.g. SIGUSR1, SIGTERM, or numeric)")
	flag.Parse()

	sig, err := signal.Parse(*sigName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[oom-signal] error: %v\n", err)
		os.Exit(1)
	}

	reader, err := memory_reader.NewMemoryReader()
	if err != nil {
		fmt.Fprintf(os.Stderr, "[oom-signal] error: %v\n", err)
		os.Exit(1)
	}

	proc, err := os.FindProcess(*targetPid)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[oom-signal] error: cannot find process %d: %v\n", *targetPid, err)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "[oom-signal] starting: threshold=%s poll-interval=%s pid=%d signal=%s reader=%s\n",
		th.String(), *pollInterval, *targetPid, signal.Name(sig), reader.Version())

	mon := monitor.New(th, *pollInterval, proc, sig, reader)
	go mon.Run()

	sigCh := make(chan os.Signal, 1)
	ossignal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)

	<-sigCh
	fmt.Fprintf(os.Stderr, "[oom-signal] received signal, stopping\n")
	mon.Stop()
}
