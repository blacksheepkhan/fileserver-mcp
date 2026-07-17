package benchmark

type processSnapshot struct {
	status              string
	workingSetBytes     *uint64
	peakWorkingSetBytes *uint64
	userCPUNS           *uint64
	systemCPUNS         *uint64
	unsupported         []string
}

type processMetricReader interface {
	Snapshot() (processSnapshot, error)
	Close() error
}

func metricPointer(value uint64) *uint64 {
	return &value
}

func unsupportedSnapshot() processSnapshot {
	return processSnapshot{
		status: "not_supported",
		unsupported: []string{
			"idle_working_set_bytes",
			"peak_working_set_bytes",
			"user_cpu_ns",
			"system_cpu_ns",
		},
	}
}
