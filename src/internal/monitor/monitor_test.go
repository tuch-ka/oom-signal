package monitor

import (
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
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

func newTestMonitor(th threshold.Threshold, reader memory_reader.MemoryReader, pollInterval, cooldown time.Duration) *Monitor {
	proc, _ := os.FindProcess(os.Getpid())
	return New(th, proc, syscall.SIGUSR1, reader, pollInterval, cooldown)
}

type signalCounter struct {
	count atomic.Int64
	ch    chan os.Signal
	done  chan struct{}
}

func newSignalCounter() *signalCounter {
	sc := &signalCounter{
		ch:   make(chan os.Signal, 64),
		done: make(chan struct{}),
	}
	signal.Notify(sc.ch, syscall.SIGUSR1)
	go func() {
		for {
			select {
			case <-sc.ch:
				sc.count.Add(1)
			case <-sc.done:
				return
			}
		}
	}()
	return sc
}

func (sc *signalCounter) get() int64 { return sc.count.Load() }
func (sc *signalCounter) reset()     { sc.count.Store(0) }
func (sc *signalCounter) stop() {
	signal.Stop(sc.ch)
	close(sc.done)
}

func TestRisingEdge_SustainedExceed_SendsOneSignal(t *testing.T) {
	sc := newSignalCounter()
	defer sc.stop()

	th, _ := threshold.New("0.8")
	reader := &mockReader{limit: 1000, usage: 500}
	mon := newTestMonitor(th, reader, 10*time.Millisecond, 0)
	go mon.Run()
	defer mon.Stop()

	time.Sleep(30 * time.Millisecond)
	sc.reset()

	reader.setUsage(900) // превысили порог
	time.Sleep(50 * time.Millisecond)

	reader.setUsage(950) // всё ещё превышен
	time.Sleep(50 * time.Millisecond)

	if got := sc.get(); got != 1 {
		t.Errorf("expected 1 signal on sustained exceed, got %d", got)
	}
}

func TestRisingEdge_Oscillation_MultipleSignals(t *testing.T) {
	sc := newSignalCounter()
	defer sc.stop()

	th, _ := threshold.New("0.8")
	reader := &mockReader{limit: 1000, usage: 500}
	mon := newTestMonitor(th, reader, 10*time.Millisecond, 0)
	go mon.Run()
	defer mon.Stop()

	time.Sleep(30 * time.Millisecond)
	sc.reset()

	for i := 0; i < 3; i++ {
		reader.setUsage(900) // превысили → rising edge
		time.Sleep(30 * time.Millisecond)
		reader.setUsage(500) // вернулось ниже порога
		time.Sleep(30 * time.Millisecond)
	}
	time.Sleep(30 * time.Millisecond)

	got := sc.get()
	if got != 3 {
		t.Errorf("expected 3 signals (one per rising edge), got %d", got)
	}
}

func TestRisingEdge_RecoveryAndReExceed(t *testing.T) {
	sc := newSignalCounter()
	defer sc.stop()

	th, _ := threshold.New("0.8")
	reader := &mockReader{limit: 1000, usage: 500}
	mon := newTestMonitor(th, reader, 10*time.Millisecond, 0)
	go mon.Run()
	defer mon.Stop()

	time.Sleep(30 * time.Millisecond)
	sc.reset()

	reader.setUsage(900) // превысили → сигнал
	time.Sleep(50 * time.Millisecond)

	reader.setUsage(500) // восстановились
	time.Sleep(50 * time.Millisecond)

	reader.setUsage(900) // снова превысили → сигнал
	time.Sleep(50 * time.Millisecond)

	if got := sc.get(); got != 2 {
		t.Errorf("expected 2 signals (recovery + re-exceed), got %d", got)
	}
}

func TestStop(t *testing.T) {
	t.Parallel()
	th, _ := threshold.New("0.8")
	reader := &mockReader{limit: 1000, usage: 500}

	mon := newTestMonitor(th, reader, 10*time.Millisecond, 0)
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

	mon := newTestMonitor(th, reader, 10*time.Millisecond, 0)
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

	mon := newTestMonitor(th, reader, 10*time.Millisecond, 0)
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
	return New(th, proc, syscall.SIGUSR1, reader, DefaultPollInterval, 0)
}

func TestCalcIntervalFirstTick(t *testing.T) {
	mon := newCalcMonitor(t)
	interval := mon.calcInterval(50*1024*1024, time.Now())
	if interval != DefaultPollInterval {
		t.Errorf("expected default on first tick, got %v", interval)
	}
}

func TestCalcIntervalNegativeDelta(t *testing.T) {
	mon := newCalcMonitor(t)
	mon.prevUsage = 80 * 1024 * 1024
	mon.prevTick = time.Now().Add(-100 * time.Millisecond)

	interval := mon.calcInterval(50*1024*1024, time.Now())
	if interval != DefaultPollInterval {
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
	if interval != DefaultPollInterval {
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
	if interval != MinPollInterval {
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

	if interval <= MinPollInterval || interval >= DefaultPollInterval {
		t.Errorf("expected interval between min and default, got %v", interval)
	}
}

func TestCalcIntervalWithCustomPollInterval(t *testing.T) {
	th, _ := threshold.New("0.8")
	proc, _ := os.FindProcess(os.Getpid())
	limit := uint64(100 * 1024 * 1024) // 100 MB
	reader := &mockReader{limit: limit}
	mon := New(th, proc, syscall.SIGUSR1, reader, 10*time.Millisecond, 0)
	mon.prevUsage = 10 * 1024 * 1024
	mon.prevTick = time.Now().Add(-100 * time.Millisecond)

	// Рост 7.5%/сек — выше highThreshold, должен вернуть MinPollInterval
	delta := uint64(7.5 * 0.1 * float64(mon.reader.Limit()) / 100)
	interval := mon.calcInterval(mon.prevUsage+delta, time.Now())
	if interval != MinPollInterval {
		t.Errorf("expected min interval with custom pollInterval, got %v", interval)
	}

	// Ниже порога — должен вернуть pollInterval
	mon.prevUsage = 10 * 1024 * 1024
	mon.prevTick = time.Now().Add(-100 * time.Millisecond)
	delta = uint64(0.3 * 0.1 * float64(mon.reader.Limit()) / 100)
	interval = mon.calcInterval(mon.prevUsage+delta, time.Now())
	if interval != mon.pollInterval {
		t.Errorf("expected pollInterval below threshold, got %v", interval)
	}
}

// --- Тесты cooldown ---

func TestCooldown_SuppressesOscillation(t *testing.T) {
	sc := newSignalCounter()
	defer sc.stop()

	th, _ := threshold.New("0.8")
	reader := &mockReader{limit: 1000, usage: 500}
	mon := newTestMonitor(th, reader, 10*time.Millisecond, 200*time.Millisecond)
	go mon.Run()
	defer mon.Stop()

	time.Sleep(30 * time.Millisecond)
	sc.reset()

	// Первое превышение → сигнал отправлен
	reader.setUsage(900)
	time.Sleep(30 * time.Millisecond)

	// Осцилляция: ниже порога и снова выше — cooldown подавляет
	for i := 0; i < 3; i++ {
		reader.setUsage(500)
		time.Sleep(20 * time.Millisecond)
		reader.setUsage(900)
		time.Sleep(20 * time.Millisecond)
	}
	time.Sleep(30 * time.Millisecond)

	// Только первый сигнал, остальные подавлены cooldown
	got := sc.get()
	if got != 1 {
		t.Errorf("expected 1 signal (rest suppressed by cooldown), got %d", got)
	}
}

func TestCooldown_AllowsAfterExpiry(t *testing.T) {
	sc := newSignalCounter()
	defer sc.stop()

	th, _ := threshold.New("0.8")
	reader := &mockReader{limit: 1000, usage: 500}
	mon := newTestMonitor(th, reader, 10*time.Millisecond, 100*time.Millisecond)
	go mon.Run()
	defer mon.Stop()

	time.Sleep(30 * time.Millisecond)
	sc.reset()

	// Превышение → сигнал
	reader.setUsage(900)
	time.Sleep(50 * time.Millisecond)

	if got := sc.get(); got != 1 {
		t.Fatalf("expected 1 signal after first exceed, got %d", got)
	}

	// Возвращаем ниже порога, ждём cooldown
	reader.setUsage(500)
	time.Sleep(150 * time.Millisecond) // cooldown истёк

	// Снова превышаем → сигнал разрешён
	reader.setUsage(900)
	time.Sleep(50 * time.Millisecond)

	if got := sc.get(); got != 2 {
		t.Errorf("expected 2 signals total (cooldown expired), got %d", got)
	}
}

func TestCooldown_Zero_Disabled(t *testing.T) {
	sc := newSignalCounter()
	defer sc.stop()

	th, _ := threshold.New("0.8")
	reader := &mockReader{limit: 1000, usage: 500}
	mon := newTestMonitor(th, reader, 10*time.Millisecond, 0)
	go mon.Run()
	defer mon.Stop()

	time.Sleep(30 * time.Millisecond)
	sc.reset()

	// Без cooldown осцилляция генерирует сигнал на каждый rising edge
	for i := 0; i < 3; i++ {
		reader.setUsage(900)
		time.Sleep(30 * time.Millisecond)
		reader.setUsage(500)
		time.Sleep(30 * time.Millisecond)
	}
	time.Sleep(30 * time.Millisecond)

	got := sc.get()
	if got != 3 {
		t.Errorf("expected 3 signals with cooldown=0, got %d", got)
	}
}
