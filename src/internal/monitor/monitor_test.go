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
	// Поглощаем SIGUSR1, чтобы монитор не убил тестовый процесс
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

func TestRisingEdgeSendsSignalOnce(t *testing.T) {
	t.Parallel()
	th, _ := threshold.New("0.8")
	proc, _ := os.FindProcess(os.Getpid())

	reader := &mockReader{
		limit: 1000,
		usage: 500,
	}

	mon := New(th, 10*time.Millisecond, proc, syscall.SIGUSR1, reader)
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
	proc, _ := os.FindProcess(os.Getpid())

	reader := &mockReader{
		limit: 1000,
		usage: 500,
	}

	mon := New(th, 10*time.Millisecond, proc, syscall.SIGUSR1, reader)
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
	proc, _ := os.FindProcess(os.Getpid())

	reader := &mockReader{
		limit: 1000,
		usage: 500,
	}

	mon := New(th, 10*time.Millisecond, proc, syscall.SIGUSR1, reader)
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
	proc, _ := os.FindProcess(os.Getpid())

	reader := &mockReader{
		limit: 1000,
		usage: 500,
		err:   os.ErrNotExist,
	}

	mon := New(th, 10*time.Millisecond, proc, syscall.SIGUSR1, reader)
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
	proc, _ := os.FindProcess(os.Getpid())

	limit := uint64(200 * 1024 * 1024)
	reader := &mockReader{
		limit: limit,
		usage: 50 * 1024 * 1024,
	}

	mon := New(th, 10*time.Millisecond, proc, syscall.SIGUSR1, reader)
	go mon.Run()
	defer mon.Stop()

	reader.setUsage(150 * 1024 * 1024)
	time.Sleep(50 * time.Millisecond)
}
