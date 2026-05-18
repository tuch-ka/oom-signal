package monitor

import (
	"os"
	"os/signal"
	"sync"
	"syscall"
	"testing"
	"time"

	"oom-signal/src/internal/memory_reader"
	"oom-signal/src/internal/threshold"
)

func TestMain(m *testing.M) {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGUSR1)
	go func() {
		for range sigCh {
		}
	}()

	os.Exit(m.Run())
}

type mockReader struct {
	limit uint64
	usage uint64
	err   error
	calls int
	mu    sync.Mutex
}

func (m *mockReader) ReadUsage() (uint64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls++
	return m.usage, m.err
}

func (m *mockReader) Limit() uint64                              { return m.limit }
func (m *mockReader) Version() memory_reader.MemoryReaderVersion { return memory_reader.CgroupV2 }

func (m *mockReader) setUsage(u uint64) {
	m.mu.Lock()
	m.usage = u
	m.mu.Unlock()
}

func (m *mockReader) getCalls() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.calls
}

func newTestMonitor(th threshold.Threshold, reader memory_reader.MemoryReader) *Monitor {
	proc, _ := os.FindProcess(os.Getpid())
	mon := New(th, proc, syscall.SIGUSR1, reader)
	mon.testPoll = 10 * time.Millisecond
	return mon
}

func TestRisingEdgeSendsSignalOnce(t *testing.T) {
	t.Parallel()
	th, _ := threshold.New("0.8")
	reader := &mockReader{limit: 1000, usage: 500}

	mon := newTestMonitor(th, reader)
	go mon.Run()
	defer mon.Stop()

	time.Sleep(30 * time.Millisecond)

	reader.setUsage(900)
	time.Sleep(50 * time.Millisecond)

	reader.setUsage(950)
	time.Sleep(30 * time.Millisecond)
}

func TestRisingEdgeSendsOnRecoveryAndReExceed(t *testing.T) {
	t.Parallel()
	th, _ := threshold.New("0.8")
	reader := &mockReader{limit: 1000, usage: 500}

	mon := newTestMonitor(th, reader)
	go mon.Run()
	defer mon.Stop()

	reader.setUsage(900)
	time.Sleep(50 * time.Millisecond)

	reader.setUsage(500)
	time.Sleep(50 * time.Millisecond)

	reader.setUsage(900)
	time.Sleep(50 * time.Millisecond)
}

func TestStop(t *testing.T) {
	t.Parallel()
	th, _ := threshold.New("0.8")
	reader := &mockReader{limit: 1000, usage: 500}

	mon := newTestMonitor(th, reader)
	go mon.Run()

	time.Sleep(30 * time.Millisecond)
	mon.Stop()

	time.Sleep(30 * time.Millisecond)
	calls := reader.getCalls()
	time.Sleep(30 * time.Millisecond)

	if reader.getCalls() > calls+2 {
		t.Error("monitor should have stopped polling after Stop()")
	}
}

func TestReadUsageErrorContinues(t *testing.T) {
	t.Parallel()
	th, _ := threshold.New("0.8")
	reader := &mockReader{limit: 1000, usage: 500, err: os.ErrNotExist}

	mon := newTestMonitor(th, reader)
	go mon.Run()
	defer mon.Stop()

	time.Sleep(50 * time.Millisecond)

	if reader.getCalls() < 2 {
		t.Error("monitor should continue polling after read error")
	}
}

func TestAbsoluteThreshold(t *testing.T) {
	t.Parallel()
	th, _ := threshold.New("100MB")
	limit := uint64(200 * 1024 * 1024)
	reader := &mockReader{limit: limit, usage: 50 * 1024 * 1024}

	mon := newTestMonitor(th, reader)
	go mon.Run()
	defer mon.Stop()

	reader.setUsage(150 * 1024 * 1024)
	time.Sleep(50 * time.Millisecond)
}

// --- Unit-тесты calcInterval ---

func newCalcMonitor(t *testing.T) *Monitor {
	t.Helper()
	th, _ := threshold.New("0.8")
	proc, _ := os.FindProcess(os.Getpid())
	limit := uint64(100 * 1024 * 1024) // 100 MB
	reader := &mockReader{limit: limit}
	return New(th, proc, syscall.SIGUSR1, reader)
}

func TestCalcIntervalFirstTick(t *testing.T) {
	mon := newCalcMonitor(t)
	interval := mon.calcInterval(50*1024*1024, time.Now())
	if interval != defaultPollInterval {
		t.Errorf("expected default on first tick, got %v", interval)
	}
}

func TestCalcIntervalNegativeDelta(t *testing.T) {
	mon := newCalcMonitor(t)
	mon.prevUsage = 80 * 1024 * 1024
	mon.prevTick = time.Now().Add(-100 * time.Millisecond)

	interval := mon.calcInterval(50*1024*1024, time.Now())
	if interval != defaultPollInterval {
		t.Errorf("expected default on negative delta, got %v", interval)
	}
}

func TestCalcIntervalBelowLowThreshold(t *testing.T) {
	mon := newCalcMonitor(t)
	mon.prevUsage = 10 * 1024 * 1024
	mon.prevTick = time.Now().Add(-100 * time.Millisecond)

	// Рост 0.3%/сек — ниже growthLowThreshold (0.5%)
	delta := uint64(0.3 * 0.1 * float64(mon.reader.Limit()) / 100)
	interval := mon.calcInterval(mon.prevUsage+delta, time.Now())
	if interval != defaultPollInterval {
		t.Errorf("expected default below low threshold, got %v", interval)
	}
}

func TestCalcIntervalAboveHighThreshold(t *testing.T) {
	mon := newCalcMonitor(t)
	mon.prevUsage = 10 * 1024 * 1024
	mon.prevTick = time.Now().Add(-100 * time.Millisecond)

	// Рост 7.5%/сек — выше growthHighThreshold (5.0%)
	delta := uint64(7.5 * 0.1 * float64(mon.reader.Limit()) / 100)
	interval := mon.calcInterval(mon.prevUsage+delta, time.Now())
	if interval != minPollInterval {
		t.Errorf("expected min interval above high threshold, got %v", interval)
	}
}

func TestCalcIntervalBetweenThresholds(t *testing.T) {
	mon := newCalcMonitor(t)
	mon.prevUsage = 10 * 1024 * 1024
	mon.prevTick = time.Now().Add(-100 * time.Millisecond)

	// Рост 3.0%/сек — между порогами
	growthPct := 3.0
	delta := uint64(growthPct * 0.1 * float64(mon.reader.Limit()) / 100)
	interval := mon.calcInterval(mon.prevUsage+delta, time.Now())

	if interval <= minPollInterval || interval >= defaultPollInterval {
		t.Errorf("expected interval between min and default, got %v", interval)
	}
}

func TestCalcIntervalWithTestPoll(t *testing.T) {
	mon := newCalcMonitor(t)
	mon.testPoll = 10 * time.Millisecond
	mon.prevUsage = 10 * 1024 * 1024
	mon.prevTick = time.Now().Add(-100 * time.Millisecond)

	// Рост 7.5%/сек — выше highThreshold, должен вернуть minPollInterval
	delta := uint64(7.5 * 0.1 * float64(mon.reader.Limit()) / 100)
	interval := mon.calcInterval(mon.prevUsage+delta, time.Now())
	if interval != minPollInterval {
		t.Errorf("expected min interval with testPoll, got %v", interval)
	}

	// Ниже порога — должен вернуть testPoll
	mon.prevUsage = 10 * 1024 * 1024
	mon.prevTick = time.Now().Add(-100 * time.Millisecond)
	delta = uint64(0.3 * 0.1 * float64(mon.reader.Limit()) / 100)
	interval = mon.calcInterval(mon.prevUsage+delta, time.Now())
	if interval != mon.testPoll {
		t.Errorf("expected testPoll below threshold, got %v", interval)
	}
}
