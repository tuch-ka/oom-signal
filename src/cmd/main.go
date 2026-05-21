package main

import (
	"flag"
	"fmt"
	"os"
	ossignal "os/signal"
	"syscall"

	"oom-signal/src/internal/memory_reader"
	"oom-signal/src/internal/monitor"
	"oom-signal/src/internal/signal"
	"oom-signal/src/internal/threshold"
)

func main() {
	th := threshold.Default()
	flag.Var(&th, "threshold", "Memory threshold: fraction 0.0–1.0 or megabytes e.g. 100MB")
	targetPid := flag.Int("pid", 1, "PID to signal when threshold is exceeded")
	sigName := flag.String("signal", "SIGUSR1", "Signal to send (e.g. SIGUSR1, SIGTERM, or numeric)")
	pollInterval := flag.Duration("poll-interval", monitor.DefaultPollInterval, "Base poll interval (minimum 1ms)")
	cooldown := flag.Duration("cooldown", monitor.DefaultCooldown, "Minimum interval between signals, 0 to disable")
	flag.Parse()

	// Валидация до NewMemoryReader: при невалидных параметрах не имеет смысла
	// пытаться определить версию cgroup.
	if *pollInterval < monitor.MinPollInterval {
		fmt.Fprintf(os.Stderr, "[oom-signal] error: poll-interval must be at least %s\n", monitor.MinPollInterval)
		os.Exit(1)
	}

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

	fmt.Fprintf(os.Stderr, "[oom-signal] starting: threshold=%s pid=%d signal=%s poll-interval=%s cooldown=%s reader=%s\n",
		th.String(), *targetPid, signal.Name(sig), pollInterval, cooldown, reader.Version())

	mon := monitor.New(th, proc, sig, reader, *pollInterval, *cooldown)
	go mon.Run()

	sigCh := make(chan os.Signal, 1)
	ossignal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)

	<-sigCh
	fmt.Fprintf(os.Stderr, "[oom-signal] received signal, stopping\n")
	mon.Stop()
}
