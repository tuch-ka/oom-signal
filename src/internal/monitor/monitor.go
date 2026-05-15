package monitor

import (
	"fmt"
	"os"
	"sync"
	"syscall"
	"time"

	"oom-signal/src/internal/memory_reader"
	"oom-signal/src/internal/threshold"
)

// Monitor periodically checks cgroup memory usage and sends a signal
// to the target process when the threshold is exceeded (rising edge).
type Monitor struct {
	threshold    threshold.Threshold
	pollInterval time.Duration
	proc         *os.Process
	sig          syscall.Signal
	reader       memory_reader.MemoryReader
	stopOnce     sync.Once
	stopCh       chan struct{}
	exceeded     bool
}

// New creates a new memory monitor.
func New(th threshold.Threshold, pollInterval time.Duration, proc *os.Process, sig syscall.Signal, reader memory_reader.MemoryReader) *Monitor {
	return &Monitor{
		threshold:    th,
		pollInterval: pollInterval,
		proc:         proc,
		sig:          sig,
		reader:       reader,
		stopCh:       make(chan struct{}),
	}
}

// Run starts the monitoring loop. It runs until Stop is called.
// A signal is sent each time the threshold is crossed (rising edge only).
func (m *Monitor) Run() {
	ticker := time.NewTicker(m.pollInterval)
	defer ticker.Stop()

	limit := m.reader.Limit()

	for {
		select {
		case <-ticker.C:
			usage, err := m.reader.ReadUsage()
			if err != nil {
				fmt.Fprintf(os.Stderr, "[oom-signal] error reading cgroup memory: %v\n", err)
				continue
			}

			nowExceeded := m.threshold.Exceeded(usage, limit)
			if nowExceeded && !m.exceeded {
				var freeStr string
				if usage > limit {
					freeStr = "0 (over limit)"
				} else {
					freeStr = fmt.Sprintf("%d", limit-usage)
				}
				fmt.Fprintf(os.Stderr, "[oom-signal] memory threshold exceeded: usage %d / limit %d (free %s bytes, threshold %s)\n",
					usage, limit, freeStr, m.threshold.LogString())
				if err := m.proc.Signal(m.sig); err != nil {
					fmt.Fprintf(os.Stderr, "[oom-signal] error sending signal to pid %d: %v\n", m.proc.Pid, err)
				} else {
					fmt.Fprintf(os.Stderr, "[oom-signal] signal sent to pid %d\n", m.proc.Pid)
				}
			}
			m.exceeded = nowExceeded

		case <-m.stopCh:
			return
		}
	}
}

// Stop signals the monitoring loop to stop. Safe to call multiple times.
func (m *Monitor) Stop() {
	m.stopOnce.Do(func() { close(m.stopCh) })
}
