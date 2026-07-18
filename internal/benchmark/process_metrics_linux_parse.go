package benchmark

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"strconv"
	"strings"
)

type linuxMemoryMetrics struct {
	workingSetBytes     *uint64
	peakWorkingSetBytes *uint64
	unsupported         []string
}

type linuxCPUMetrics struct {
	userCPUNS   *uint64
	systemCPUNS *uint64
	unsupported []string
}

func parseLinuxStatus(reader io.Reader) (linuxMemoryMetrics, error) {
	metrics := linuxMemoryMetrics{}
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 2 || (fields[0] != "VmRSS:" && fields[0] != "VmHWM:") {
			continue
		}
		value, err := strconv.ParseUint(fields[1], 10, 64)
		if err != nil {
			continue
		}
		bytesValue := value * 1024
		switch fields[0] {
		case "VmRSS:":
			metrics.workingSetBytes = metricPointer(bytesValue)
		case "VmHWM:":
			metrics.peakWorkingSetBytes = metricPointer(bytesValue)
		}
	}
	if err := scanner.Err(); err != nil {
		return linuxMemoryMetrics{}, fmt.Errorf("read proc status: %w", err)
	}
	if metrics.workingSetBytes == nil {
		metrics.unsupported = append(metrics.unsupported, "idle_working_set_bytes")
	}
	if metrics.peakWorkingSetBytes == nil {
		metrics.unsupported = append(metrics.unsupported, "peak_working_set_bytes")
	}
	return metrics, nil
}

func parseLinuxStat(data []byte, ticksPerSecond uint64) (linuxCPUMetrics, error) {
	if ticksPerSecond == 0 {
		return linuxCPUMetrics{}, fmt.Errorf("invalid proc tick rate")
	}
	closingParen := bytes.LastIndexByte(data, ')')
	if closingParen < 0 || closingParen+2 >= len(data) {
		return linuxCPUMetrics{}, fmt.Errorf("invalid proc stat format")
	}
	fields := strings.Fields(string(data[closingParen+2:]))
	if len(fields) <= 11 {
		return linuxCPUMetrics{}, fmt.Errorf("proc stat omitted CPU fields")
	}
	metrics := linuxCPUMetrics{}
	userTicks, userErr := strconv.ParseUint(fields[11], 10, 64)
	if userErr != nil {
		metrics.unsupported = append(metrics.unsupported, "user_cpu_ns")
	} else {
		metrics.userCPUNS = metricPointer(userTicks * 1_000_000_000 / ticksPerSecond)
	}
	if len(fields) <= 12 {
		metrics.unsupported = append(metrics.unsupported, "system_cpu_ns")
	} else {
		systemTicks, systemErr := strconv.ParseUint(fields[12], 10, 64)
		if systemErr != nil {
			metrics.unsupported = append(metrics.unsupported, "system_cpu_ns")
		} else {
			metrics.systemCPUNS = metricPointer(systemTicks * 1_000_000_000 / ticksPerSecond)
		}
	}
	if metrics.userCPUNS == nil && metrics.systemCPUNS == nil {
		return metrics, fmt.Errorf("proc stat CPU fields are invalid")
	}
	return metrics, nil
}
