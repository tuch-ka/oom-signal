package memory_reader

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseCgroupFileV2(t *testing.T) {
	t.Parallel()
	data := []byte("0::/kubepods/besteffort/pod123\n")
	ver, path := parseCgroupFile(data)
	if ver != CgroupV2 {
		t.Errorf("expected CgroupV2, got %v", ver)
	}
	if path != "/kubepods/besteffort/pod123" {
		t.Errorf("expected /kubepods/besteffort/pod123, got %s", path)
	}
}

func TestParseCgroupFileV1(t *testing.T) {
	t.Parallel()
	data := []byte("10:memory:/kubepods/burstable/pod456\n")
	ver, path := parseCgroupFile(data)
	if ver != CgroupV1 {
		t.Errorf("expected CgroupV1, got %v", ver)
	}
	if path != "/kubepods/burstable/pod456" {
		t.Errorf("expected /kubepods/burstable/pod456, got %s", path)
	}
}

func TestParseCgroupFileV1Nested(t *testing.T) {
	t.Parallel()
	data := []byte("10:memory:/kubepods/burstable/podabc/container123\n1:cpuset:/\n")
	ver, path := parseCgroupFile(data)
	if ver != CgroupV1 {
		t.Errorf("expected CgroupV1, got %v", ver)
	}
	if path != "/kubepods/burstable/podabc/container123" {
		t.Errorf("expected nested path, got %s", path)
	}
}

func TestParseCgroupFileV2BeforeV1(t *testing.T) {
	t.Parallel()
	data := []byte("0::/system.slice/app.service\n10:memory:/app\n")
	ver, path := parseCgroupFile(data)
	if ver != CgroupV2 {
		t.Errorf("expected CgroupV2 to take priority, got %v", ver)
	}
	if path != "/system.slice/app.service" {
		t.Errorf("expected /system.slice/app.service, got %s", path)
	}
}

func TestParseCgroupFileEmpty(t *testing.T) {
	t.Parallel()
	ver, path := parseCgroupFile([]byte(""))
	if ver != MemoryReaderUnknown {
		t.Errorf("expected MemoryReaderUnknown, got %v", ver)
	}
	if path != "" {
		t.Errorf("expected empty path, got %s", path)
	}
}

func TestParseCgroupFileNoMatch(t *testing.T) {
	t.Parallel()
	data := []byte("1:cpuset:/\n2:cpu:/\n")
	ver, _ := parseCgroupFile(data)
	if ver != MemoryReaderUnknown {
		t.Errorf("expected MemoryReaderUnknown, got %v", ver)
	}
}

func TestParseCgroupFileTrailingNewline(t *testing.T) {
	t.Parallel()
	data := []byte("0::/app\n\n")
	ver, path := parseCgroupFile(data)
	if ver != CgroupV2 {
		t.Errorf("expected CgroupV2, got %v", ver)
	}
	if path != "/app" {
		t.Errorf("expected /app, got %s", path)
	}
}

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
