package signal

import (
	"fmt"
	"strconv"
	"strings"
	"syscall"
)

var signalNames = map[string]syscall.Signal{
	"SIGHUP":  syscall.SIGHUP,
	"SIGINT":  syscall.SIGINT,
	"SIGQUIT": syscall.SIGQUIT,
	"SIGILL":  syscall.SIGILL,
	"SIGTRAP": syscall.SIGTRAP,
	"SIGABRT": syscall.SIGABRT,
	"SIGBUS":  syscall.SIGBUS,
	"SIGFPE":  syscall.SIGFPE,
	"SIGKILL": syscall.SIGKILL,
	"SIGUSR1": syscall.SIGUSR1,
	"SIGSEGV": syscall.SIGSEGV,
	"SIGUSR2": syscall.SIGUSR2,
	"SIGPIPE": syscall.SIGPIPE,
	"SIGALRM": syscall.SIGALRM,
	"SIGTERM": syscall.SIGTERM,
	"SIGCHLD": syscall.SIGCHLD,
	"SIGCONT": syscall.SIGCONT,
	"SIGSTOP": syscall.SIGSTOP,
	"SIGTSTP": syscall.SIGTSTP,
	"SIGTTIN": syscall.SIGTTIN,
	"SIGTTOU": syscall.SIGTTOU,
}

var signalByValue = map[syscall.Signal]string{}

func init() {
	for name, sig := range signalNames {
		signalByValue[sig] = name
	}
}

// Parse parses a signal from a string. Accepts signal names (e.g. "SIGUSR1")
// or numeric values (e.g. "10"). Numeric values are passed through without
// validation — the OS will reject invalid signals at send time.
func Parse(s string) (syscall.Signal, error) {
	if sig, ok := signalNames[strings.ToUpper(s)]; ok {
		return sig, nil
	}

	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("invalid signal %q", s)
	}
	return syscall.Signal(n), nil
}

// Name returns the signal name for logging. Known signals return their
// canonical name (e.g. "SIGUSR1"), unknown numeric values return "signal N".
func Name(sig syscall.Signal) string {
	if name, ok := signalByValue[sig]; ok {
		return name
	}
	return fmt.Sprintf("signal %d", sig)
}
