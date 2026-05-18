package memory_reader

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMemoryReaderVersionString(t *testing.T) {
	t.Parallel()
	cases := []struct {
		v    MemoryReaderVersion
		want string
	}{
		{CgroupV1, "cgroup v1"},
		{CgroupV2, "cgroup v2"},
		{MemoryReaderUnknown, "unknown"},
		{MemoryReaderVersion(99), "unknown"},
	}
	for _, c := range cases {
		if got := c.v.String(); got != c.want {
			t.Errorf("MemoryReaderVersion(%d).String() = %q, want %q", c.v, got, c.want)
		}
	}
}

func TestNewMemoryReaderV1Probed(t *testing.T) {
	dir := t.TempDir()

	// Подменяем пути cgroup v1 на временные
	origMountV1 := cgroupMountV1
	cgroupMountV1 = dir
	t.Cleanup(func() { cgroupMountV1 = origMountV1 })

	writeFile(t, dir+"/"+cgroupV1UsageFile, "536870912\n")
	writeFile(t, dir+"/"+cgroupV1LimitFile, "1073741824\n")

	r, err := NewMemoryReader()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.Version() != CgroupV1 {
		t.Errorf("expected CgroupV1, got %v", r.Version())
	}
	if r.Limit() != 1073741824 {
		t.Errorf("expected limit 1073741824, got %d", r.Limit())
	}
}

func TestNewMemoryReaderV2Probed(t *testing.T) {
	dir := t.TempDir()

	// Подменяем пути cgroup v2 на временные
	origMountV2 := cgroupMountV2
	cgroupMountV2 = dir
	t.Cleanup(func() { cgroupMountV2 = origMountV2 })

	writeFile(t, dir+"/"+cgroupV2UsageFile, "536870912\n")
	writeFile(t, dir+"/"+cgroupV2LimitFile, "2147483648\n")

	r, err := NewMemoryReader()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.Version() != CgroupV2 {
		t.Errorf("expected CgroupV2, got %v", r.Version())
	}
	if r.Limit() != 2147483648 {
		t.Errorf("expected limit 2147483648, got %d", r.Limit())
	}
}

func TestNewMemoryReaderV1PriorityOverV2(t *testing.T) {
	dir := t.TempDir()

	origMountV1 := cgroupMountV1
	origMountV2 := cgroupMountV2
	cgroupMountV1 = dir + "/v1"
	cgroupMountV2 = dir + "/v2"
	t.Cleanup(func() {
		cgroupMountV1 = origMountV1
		cgroupMountV2 = origMountV2
	})

	writeFile(t, dir+"/v1/"+cgroupV1UsageFile, "100\n")
	writeFile(t, dir+"/v1/"+cgroupV1LimitFile, "200\n")
	writeFile(t, dir+"/v2/"+cgroupV2UsageFile, "300\n")
	writeFile(t, dir+"/v2/"+cgroupV2LimitFile, "400\n")

	r, err := NewMemoryReader()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.Version() != CgroupV1 {
		t.Errorf("expected CgroupV1 (priority), got %v", r.Version())
	}
}

func TestNewMemoryReaderNoneAvailable(t *testing.T) {
	_, err := NewMemoryReader()
	if err == nil {
		t.Error("expected error when no cgroup available")
	}
}

func TestReadV1LimitUnlimited(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := dir + "/limit"
	writeFile(t, path, "9223372036854775807")

	_, err := readV1Limit(path)
	if err == nil {
		t.Error("expected error for unlimited v1 limit")
	}
}

func TestReadV1LimitValid(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := dir + "/limit"
	writeFile(t, path, "1073741824")

	limit, err := readV1Limit(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if limit != 1073741824 {
		t.Errorf("expected 1073741824, got %d", limit)
	}
}

func TestReadV1LimitMissingFile(t *testing.T) {
	t.Parallel()
	_, err := readV1Limit("/nonexistent/path")
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestReadV2LimitMax(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := dir + "/max"
	writeFile(t, path, "max")

	_, err := readV2Limit(path)
	if err == nil {
		t.Error("expected error for v2 max limit")
	}
}

func TestReadV2LimitValid(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := dir + "/max"
	writeFile(t, path, "2147483648")

	limit, err := readV2Limit(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if limit != 2147483648 {
		t.Errorf("expected 2147483648, got %d", limit)
	}
}

func TestReadV2LimitMissingFile(t *testing.T) {
	t.Parallel()
	_, err := readV2Limit("/nonexistent/path")
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestReadUintFromFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := dir + "/usage"
	writeFile(t, path, "536870912\n")

	val, err := readUintFromFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != 536870912 {
		t.Errorf("expected 536870912, got %d", val)
	}
}

func TestReadUintFromFileMissing(t *testing.T) {
	t.Parallel()
	_, err := readUintFromFile("/nonexistent/path")
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestReadStringFromFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := dir + "/data"
	writeFile(t, path, "  hello  \n")

	val, err := readStringFromFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != "hello" {
		t.Errorf("expected 'hello', got %q", val)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
}
