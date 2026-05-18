package memory_reader

import "fmt"

const noLimitV1 uint64 = 1 << 62

var (
	cgroupMountV1     = "/sys/fs/cgroup/memory"
	cgroupV1UsageFile = "memory.usage_in_bytes"
	cgroupV1LimitFile = "memory.limit_in_bytes"
)

type cgroupV1 struct {
	limit     uint64
	usagePath string
}

func newCgroupReaderV1(cgroupPath string) (*cgroupV1, error) {
	base := cgroupMountV1 + cgroupPath

	usagePath := base + "/" + cgroupV1UsageFile
	limitPath := base + "/" + cgroupV1LimitFile

	limit, err := readV1Limit(limitPath)
	if err != nil {
		return nil, err
	}

	return &cgroupV1{
		limit:     limit,
		usagePath: usagePath,
	}, nil
}

func readV1Limit(path string) (uint64, error) {
	limit, err := readUintFromFile(path)
	if err != nil {
		return 0, fmt.Errorf("read v1 limit: %w", err)
	}
	if limit >= noLimitV1 {
		return 0, fmt.Errorf("memory limit is not set (cgroup v1 limit is unlimited)")
	}
	return limit, nil
}

func (r *cgroupV1) ReadUsage() (uint64, error) {
	usage, err := readUintFromFile(r.usagePath)
	if err != nil {
		return 0, fmt.Errorf("read v1 usage: %w", err)
	}
	return usage, nil
}

func (r *cgroupV1) Limit() uint64                { return r.limit }
func (r *cgroupV1) Version() MemoryReaderVersion { return CgroupV1 }
