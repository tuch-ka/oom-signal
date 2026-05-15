package memory_reader

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// MemoryReaderVersion represents the detected cgroup version.
type MemoryReaderVersion int

const (
	MemoryReaderUnknown MemoryReaderVersion = iota
	CgroupV1
	CgroupV2
)

func (v MemoryReaderVersion) String() string {
	switch v {
	case CgroupV1:
		return "cgroup v1"
	case CgroupV2:
		return "cgroup v2"
	default:
		return "unknown"
	}
}

// MemoryReader provides memory usage and limit information.
type MemoryReader interface {
	ReadUsage() (uint64, error)
	Limit() uint64
	Version() MemoryReaderVersion
}

// NewMemoryReader detects the cgroup version, creates the appropriate reader,
// and verifies that a memory limit is set. Returns an error if cgroup cannot
// be detected or if the memory limit is unlimited.
func NewMemoryReader() (MemoryReader, error) {
	data, err := os.ReadFile(procSelfCgroup)
	if err == nil {
		if ver, basePath := parseCgroupFile(data); ver != MemoryReaderUnknown {
			return newForVersion(ver, basePath)
		}
	}

	// Fallback: stat-based detection for standard mount points.
	if _, err := os.Stat(cgroupMountV2 + "/" + cgroupV2UsageFile); err == nil {
		return newForVersion(CgroupV2, "")
	}
	if _, err := os.Stat(cgroupMountV1 + "/" + cgroupV1UsageFile); err == nil {
		return newForVersion(CgroupV1, "")
	}

	return nil, fmt.Errorf("cannot detect cgroup")
}

func newForVersion(ver MemoryReaderVersion, basePath string) (MemoryReader, error) {
	switch ver {
	case CgroupV2:
		return newCgroupReaderV2(basePath)
	case CgroupV1:
		return newCgroupReaderV1(basePath)
	default:
		return nil, fmt.Errorf("unknown cgroup version")
	}
}

const procSelfCgroup = "/proc/self/cgroup"

func parseCgroupFile(data []byte) (MemoryReaderVersion, string) {
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		if strings.HasPrefix(line, "0::") {
			return CgroupV2, line[3:]
		}

		if idx := strings.Index(line, ":memory:"); idx >= 0 {
			return CgroupV1, line[idx+len(":memory:"):]
		}
	}
	return MemoryReaderUnknown, ""
}

func readUintFromFile(path string) (uint64, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	return strconv.ParseUint(strings.TrimSpace(string(data)), 10, 64)
}

func readStringFromFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}
