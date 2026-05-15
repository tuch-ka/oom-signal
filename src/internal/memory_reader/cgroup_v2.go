package memory_reader

import (
	"fmt"
	"strconv"
)

const (
	cgroupMountV2     = "/sys/fs/cgroup"
	cgroupV2UsageFile = "memory.current"
	cgroupV2LimitFile = "memory.max"
)

type cgroupV2 struct {
	limit     uint64
	usagePath string
}

func newCgroupReaderV2(cgroupPath string) (*cgroupV2, error) {
	base := cgroupMountV2 + cgroupPath

	usagePath := base + "/" + cgroupV2UsageFile
	limitPath := base + "/" + cgroupV2LimitFile

	limit, err := readV2Limit(limitPath)
	if err != nil {
		return nil, err
	}

	return &cgroupV2{
		limit:     limit,
		usagePath: usagePath,
	}, nil
}

func readV2Limit(path string) (uint64, error) {
	raw, err := readStringFromFile(path)
	if err != nil {
		return 0, fmt.Errorf("read v2 limit: %w", err)
	}
	if raw == "max" {
		return 0, fmt.Errorf("memory limit is not set (cgroup v2 limit is \"max\")")
	}
	limit, err := strconv.ParseUint(raw, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse v2 limit %q: %w", raw, err)
	}
	return limit, nil
}

func (r *cgroupV2) ReadUsage() (uint64, error) {
	usage, err := readUintFromFile(r.usagePath)
	if err != nil {
		return 0, fmt.Errorf("read v2 usage: %w", err)
	}
	return usage, nil
}

func (r *cgroupV2) Limit() uint64                { return r.limit }
func (r *cgroupV2) Version() MemoryReaderVersion { return CgroupV2 }
