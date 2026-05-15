package signal

import (
	"strconv"
	"syscall"
	"testing"
)

func TestParseName(t *testing.T) {
	t.Parallel()
	sig, err := Parse("SIGUSR1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sig != syscall.SIGUSR1 {
		t.Errorf("expected SIGUSR1, got %v", sig)
	}
}

func TestParseNameLowercase(t *testing.T) {
	t.Parallel()
	sig, err := Parse("sigterm")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sig != syscall.SIGTERM {
		t.Errorf("expected SIGTERM, got %v", sig)
	}
}

func TestParseNumeric(t *testing.T) {
	t.Parallel()
	sig, err := Parse("10")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sig != syscall.Signal(10) {
		t.Errorf("expected signal 10, got %v", sig)
	}
}

func TestParseNumericPassThrough(t *testing.T) {
	t.Parallel()
	sig, err := Parse("99")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sig != syscall.Signal(99) {
		t.Errorf("expected signal 99, got %v", sig)
	}
}

func TestParseInvalid(t *testing.T) {
	t.Parallel()
	_, err := Parse("notasignal")
	if err == nil {
		t.Error("expected error for invalid signal")
	}
}

func TestNameKnown(t *testing.T) {
	t.Parallel()
	name := Name(syscall.SIGUSR1)
	if name != "SIGUSR1" {
		t.Errorf("expected SIGUSR1, got %s", name)
	}
}

func TestNameUnknown(t *testing.T) {
	t.Parallel()
	name := Name(syscall.Signal(99))
	expected := "signal " + strconv.Itoa(99)
	if name != expected {
		t.Errorf("expected %s, got %s", expected, name)
	}
}
