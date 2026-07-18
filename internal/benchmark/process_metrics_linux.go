//go:build linux

package benchmark

import (
	"fmt"
	"os"
)

// Linux exposes process CPU accounting in USER_HZ units through /proc/<pid>/stat.
// USER_HZ is 100 for the supported Linux procfs ABI.
const linuxUserHZ = 100

type linuxProcessMetricReader struct {
	pid int
}

func newProcessMetricReader(pid int) (processMetricReader, error) {
	return &linuxProcessMetricReader{pid: pid}, nil
}

func (r *linuxProcessMetricReader) Snapshot() (processSnapshot, error) {
	snapshot := processSnapshot{status: "not_supported"}
	memory, memoryErr := readLinuxMemory(r.pid)
	if memoryErr != nil {
		snapshot.unsupported = append(snapshot.unsupported, "idle_working_set_bytes", "peak_working_set_bytes")
	} else {
		snapshot.workingSetBytes = memory.workingSetBytes
		snapshot.peakWorkingSetBytes = memory.peakWorkingSetBytes
		snapshot.unsupported = append(snapshot.unsupported, memory.unsupported...)
	}
	cpu, cpuErr := readLinuxCPU(r.pid)
	if cpuErr != nil {
		snapshot.unsupported = append(snapshot.unsupported, "user_cpu_ns", "system_cpu_ns")
	} else {
		snapshot.userCPUNS = cpu.userCPUNS
		snapshot.systemCPUNS = cpu.systemCPUNS
		snapshot.unsupported = append(snapshot.unsupported, cpu.unsupported...)
	}
	snapshot.unsupported = uniqueSorted(snapshot.unsupported)
	if snapshot.workingSetBytes != nil || snapshot.peakWorkingSetBytes != nil || snapshot.userCPUNS != nil || snapshot.systemCPUNS != nil {
		snapshot.status = "supported"
		return snapshot, nil
	}
	return snapshot, fmt.Errorf("procfs resource metrics unavailable: memory: %v; CPU: %v", memoryErr, cpuErr)
}

func (r *linuxProcessMetricReader) Close() error { return nil }

func readLinuxMemory(pid int) (linuxMemoryMetrics, error) {
	file, err := os.Open(fmt.Sprintf("/proc/%d/status", pid))
	if err != nil {
		return linuxMemoryMetrics{}, fmt.Errorf("open proc status: %w", err)
	}
	defer file.Close()
	return parseLinuxStatus(file)
}

func readLinuxCPU(pid int) (linuxCPUMetrics, error) {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return linuxCPUMetrics{}, fmt.Errorf("read proc stat: %w", err)
	}
	return parseLinuxStat(data, linuxUserHZ)
}
