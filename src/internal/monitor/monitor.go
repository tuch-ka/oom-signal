package monitor

import (
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"oom-signal/src/internal/memory_reader"
	"oom-signal/src/internal/threshold"
)

const (
	DefaultPollInterval = 100 * time.Millisecond
	MinPollInterval     = 1 * time.Millisecond
	DefaultCooldown     = 5 * time.Second

	// Пороги скорости роста (% от limit в секунду).
	growthLowThreshold  = 0.5
	growthHighThreshold = 5.0
)

// Monitor periodically checks cgroup memory usage and sends a signal
// to the target process when the threshold is exceeded (rising edge).
type Monitor struct {
	threshold    threshold.Threshold
	proc         *os.Process
	sig          syscall.Signal
	reader       memory_reader.MemoryReader
	stopOnce     sync.Once
	stopCh       chan struct{}
	exceeded     bool
	prevUsage    uint64
	prevTick     time.Time
	curInterval  atomic.Int64
	pollInterval time.Duration
	cooldown     time.Duration
	lastSignal   time.Time
}

// calcInterval рассчитывает интервал опроса на основе скорости роста usage.
func (m *Monitor) calcInterval(usage uint64, now time.Time) time.Duration {
	def := m.pollInterval

	if m.prevTick.IsZero() {
		return def
	}

	delta := int64(usage) - int64(m.prevUsage)
	if delta <= 0 {
		return def
	}

	elapsed := now.Sub(m.prevTick).Seconds()
	if elapsed <= 0 {
		return def
	}

	growthPct := float64(delta) / elapsed / float64(m.reader.Limit()) * 100

	if growthPct <= growthLowThreshold {
		return def
	}
	if growthPct >= growthHighThreshold {
		return MinPollInterval
	}

	fraction := (growthPct - growthLowThreshold) / (growthHighThreshold - growthLowThreshold)
	return def - time.Duration(fraction*float64(def-MinPollInterval))
}

// New creates a new memory monitor.
func New(th threshold.Threshold, proc *os.Process, sig syscall.Signal, reader memory_reader.MemoryReader, pollInterval, cooldown time.Duration) *Monitor {
	return &Monitor{
		threshold:    th,
		proc:         proc,
		sig:          sig,
		reader:       reader,
		stopCh:       make(chan struct{}),
		pollInterval: pollInterval,
		cooldown:     cooldown,
	}
}

// Run starts the monitoring loop. It runs until Stop is called.
// A signal is sent each time the threshold is crossed (rising edge only).
func (m *Monitor) Run() {
	def := m.pollInterval
	m.curInterval.Store(int64(def))

	ticker := time.NewTicker(def)
	defer ticker.Stop()

	limit := m.reader.Limit()

	for {
		select {
		case now := <-ticker.C:
			usage, err := m.reader.ReadUsage()
			if err != nil {
				fmt.Fprintf(os.Stderr, "[oom-signal] error reading cgroup memory: %v\n", err)
				continue
			}

			newInterval := m.calcInterval(usage, now)
			if newInterval != time.Duration(m.curInterval.Load()) {
				fmt.Fprintf(os.Stderr, "[oom-signal] poll interval: %s → %s\n",
					time.Duration(m.curInterval.Load()), newInterval)
				m.curInterval.Store(int64(newInterval))
				ticker.Reset(newInterval)
			}

			m.prevUsage = usage
			m.prevTick = now

			nowExceeded := m.threshold.Exceeded(usage, limit)
			if nowExceeded && !m.exceeded {
				if m.cooldown > 0 && !m.lastSignal.IsZero() && now.Sub(m.lastSignal) < m.cooldown {
					fmt.Fprintf(os.Stderr, "[oom-signal] signal suppressed: cooldown (%s not elapsed since last signal)\n", m.cooldown)
				} else {
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
					m.lastSignal = now
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
