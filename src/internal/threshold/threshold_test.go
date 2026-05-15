package threshold

import "testing"

func TestNewPercent(t *testing.T) {
	t.Parallel()
	th, err := New("0.75")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if th.String() != "0.75" {
		t.Errorf("expected 0.75, got %s", th.String())
	}
	if !th.Exceeded(900, 1000) {
		t.Error("should be exceeded at 90%")
	}
	if th.Exceeded(700, 1000) {
		t.Error("should not be exceeded at 70%")
	}
}

func TestNewMegabytes(t *testing.T) {
	t.Parallel()
	th, err := New("100MB")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if th.String() != "100MB" {
		t.Errorf("expected 100MB, got %s", th.String())
	}

	limit := uint64(200 * 1024 * 1024) // 200MB
	usage := uint64(150 * 1024 * 1024) // 150MB, free = 50MB < 100MB

	if !th.Exceeded(usage, limit) {
		t.Error("should be exceeded when free < 100MB")
	}

	usage2 := uint64(50 * 1024 * 1024) // 50MB, free = 150MB > 100MB
	if th.Exceeded(usage2, limit) {
		t.Error("should not be exceeded when free >= 100MB")
	}
}

func TestNewMegabytesShortSuffix(t *testing.T) {
	t.Parallel()
	th, err := New("50M")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if th.String() != "50MB" {
		t.Errorf("expected 50MB, got %s", th.String())
	}
}

func TestNewInvalid(t *testing.T) {
	t.Parallel()
	cases := []string{"-1", "0", "1.5", "bad", "0MB"}
	for _, c := range cases {
		_, err := New(c)
		if err == nil {
			t.Errorf("expected error for %q", c)
		}
	}
}

func TestDefaultReturns08(t *testing.T) {
	t.Parallel()
	th := Default()
	if th.String() != "0.80" {
		t.Errorf("expected 0.80, got %s", th.String())
	}
}

func TestExceededUsageOverLimit(t *testing.T) {
	t.Parallel()
	th, _ := New("100MB")
	if !th.Exceeded(300, 200) {
		t.Error("should be exceeded when usage > limit")
	}
}

func TestLogString(t *testing.T) {
	t.Parallel()
	th1, _ := New("0.8")
	if th1.LogString() != "80%" {
		t.Errorf("expected 80%%, got %s", th1.LogString())
	}

	th2, _ := New("50MB")
	if th2.LogString() != "< 50MB free" {
		t.Errorf("expected '< 50MB free', got %s", th2.LogString())
	}
}
