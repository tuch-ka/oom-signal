package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

var (
	buildOnce sync.Once
	binPath   string
	binErr    error
)

func TestMain(m *testing.M) {
	buildOnce.Do(func() {
		bin := filepath.Join(os.TempDir(), "oom-signal-test")
		cmd := exec.Command("go", "build", "-o", bin, ".")
		cmd.Dir = filepath.Join(projectRoot(), "src", "cmd")
		if out, err := cmd.CombinedOutput(); err != nil {
			binErr = fmt.Errorf("build failed: %v\n%s", err, out)
			return
		}
		binPath = bin
	})
	if binErr != nil {
		fmt.Fprintf(os.Stderr, "%v\n", binErr)
		os.Exit(1)
	}
	defer os.Remove(binPath)
	os.Exit(m.Run())
}

func TestMainInvalidSignal(t *testing.T) {
	t.Parallel()
	cmd := exec.Command(binPath, "--signal=invalid")
	out, _ := cmd.CombinedOutput()
	if !strings.Contains(string(out), "[oom-signal] error: invalid signal") {
		t.Errorf("expected invalid signal error, got: %s", out)
	}
}

func TestMainPollIntervalTooSmall(t *testing.T) {
	t.Parallel()
	cmd := exec.Command(binPath, "--poll-interval=500us")
	out, _ := cmd.CombinedOutput()
	if !strings.Contains(string(out), "[oom-signal] error: poll-interval must be at least") {
		t.Errorf("expected poll-interval validation error, got: %s", out)
	}
}

func TestMainHelp(t *testing.T) {
	t.Parallel()
	cmd := exec.Command(binPath, "--help")
	out, _ := cmd.CombinedOutput()
	s := string(out)
	if !strings.Contains(s, "-threshold") {
		t.Errorf("expected -threshold in help output, got: %s", out)
	}
	if !strings.Contains(s, "-signal") {
		t.Errorf("expected -signal in help output, got: %s", out)
	}
}

func projectRoot() string {
	wd, _ := os.Getwd()
	dir := wd
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return wd
		}
		dir = parent
	}
}
