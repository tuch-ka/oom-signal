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

// NewMemoryReader пытается создать reader перебором: сначала cgroup v1,
// затем v2. Порядок приоритета захардкожен.
func NewMemoryReader() (MemoryReader, error) {
	if r, err := newCgroupReaderV1(""); err == nil {
		return r, nil
	}
	if r, err := newCgroupReaderV2(""); err == nil {
		return r, nil
	}
	return nil, fmt.Errorf("cannot detect cgroup")
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
