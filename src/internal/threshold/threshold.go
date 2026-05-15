package threshold

import (
	"fmt"
	"strconv"
	"strings"
)

const mb = 1024 * 1024

type thresholdMode int

const (
	thresholdPercent thresholdMode = iota
	thresholdAbsolute
)

// Threshold represents a memory threshold in either percent (0.0–1.0) or
// absolute megabytes (e.g. "100MB"). It implements flag.Value.
type Threshold struct {
	mode    thresholdMode
	percent float64
	bytes   uint64
}

// New creates a Threshold from a string value (e.g. "0.8" or "100MB").
func New(value string) (Threshold, error) {
	var t Threshold
	if err := t.Set(value); err != nil {
		return t, err
	}
	return t, nil
}

func Default() Threshold {
	t, err := New("0.8")
	if err != nil {
		panic(err)
	}
	return t
}

// Exceeded returns true if the memory usage crosses the threshold.
func (t Threshold) Exceeded(usage, limit uint64) bool {
	switch t.mode {
	case thresholdPercent:
		return usage > 0 && limit > 0 && float64(usage)/float64(limit) > t.percent
	case thresholdAbsolute:
		if usage > limit {
			return true
		}
		return limit-usage < t.bytes
	default:
		return false
	}
}

func (t Threshold) String() string {
	switch t.mode {
	case thresholdPercent:
		return fmt.Sprintf("%.2f", t.percent)
	case thresholdAbsolute:
		return fmt.Sprintf("%dMB", t.bytes/mb)
	default:
		return ""
	}
}

// LogString returns a human-readable description for log output.
func (t Threshold) LogString() string {
	switch t.mode {
	case thresholdPercent:
		return fmt.Sprintf("%.0f%%", t.percent*100)
	case thresholdAbsolute:
		return fmt.Sprintf("< %dMB free", t.bytes/mb)
	default:
		return "unknown"
	}
}

// Set implements flag.Value.
func (t *Threshold) Set(s string) error {
	lower := strings.ToLower(s)

	if strings.HasSuffix(lower, "mb") || strings.HasSuffix(lower, "m") {
		suffix := 2
		if strings.HasSuffix(lower, "m") && !strings.HasSuffix(lower, "mb") {
			suffix = 1
		}
		numStr := s[:len(s)-suffix]
		val, err := strconv.ParseFloat(numStr, 64)
		if err != nil {
			return fmt.Errorf("invalid megabyte value %q: %w", numStr, err)
		}
		if val <= 0 {
			return fmt.Errorf("megabyte value must be positive, got %v", val)
		}
		t.mode = thresholdAbsolute
		t.bytes = uint64(val * float64(mb))
		return nil
	}

	val, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return fmt.Errorf("invalid threshold %q: %w", s, err)
	}
	if val <= 0 || val > 1 {
		return fmt.Errorf("percent threshold must be in (0.0, 1.0], got %v", val)
	}
	t.mode = thresholdPercent
	t.percent = val
	return nil
}
