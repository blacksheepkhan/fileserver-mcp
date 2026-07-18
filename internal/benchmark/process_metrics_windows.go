//go:build windows

package benchmark

import (
	"fmt"
	"syscall"
	"unsafe"
)

const (
	processQueryInformation = 0x0400
	processVMRead           = 0x0010
)

var (
	kernel32DLL          = syscall.NewLazyDLL("kernel32.dll")
	psapiDLL             = syscall.NewLazyDLL("psapi.dll")
	openProcessProc      = kernel32DLL.NewProc("OpenProcess")
	getProcessTimesProc  = kernel32DLL.NewProc("GetProcessTimes")
	getProcessMemoryProc = psapiDLL.NewProc("GetProcessMemoryInfo")
)

type windowsProcessMetricReader struct {
	handle syscall.Handle
}

type processMemoryCounters struct {
	CB                         uint32
	PageFaultCount             uint32
	PeakWorkingSetSize         uintptr
	WorkingSetSize             uintptr
	QuotaPeakPagedPoolUsage    uintptr
	QuotaPagedPoolUsage        uintptr
	QuotaPeakNonPagedPoolUsage uintptr
	QuotaNonPagedPoolUsage     uintptr
	PagefileUsage              uintptr
	PeakPagefileUsage          uintptr
}

func newProcessMetricReader(pid int) (processMetricReader, error) {
	handle, _, callErr := openProcessProc.Call(
		processQueryInformation|processVMRead,
		0,
		uintptr(uint32(pid)),
	)
	if handle == 0 {
		return nil, fmt.Errorf("OpenProcess: %w", callErr)
	}
	return &windowsProcessMetricReader{handle: syscall.Handle(handle)}, nil
}

func (r *windowsProcessMetricReader) Snapshot() (processSnapshot, error) {
	counters := processMemoryCounters{CB: uint32(unsafe.Sizeof(processMemoryCounters{}))}
	result, _, callErr := getProcessMemoryProc.Call(
		uintptr(r.handle),
		uintptr(unsafe.Pointer(&counters)),
		uintptr(counters.CB),
	)
	if result == 0 {
		return processSnapshot{}, fmt.Errorf("GetProcessMemoryInfo: %w", callErr)
	}

	var creationTime syscall.Filetime
	var exitTime syscall.Filetime
	var kernelTime syscall.Filetime
	var userTime syscall.Filetime
	result, _, callErr = getProcessTimesProc.Call(
		uintptr(r.handle),
		uintptr(unsafe.Pointer(&creationTime)),
		uintptr(unsafe.Pointer(&exitTime)),
		uintptr(unsafe.Pointer(&kernelTime)),
		uintptr(unsafe.Pointer(&userTime)),
	)
	if result == 0 {
		return processSnapshot{}, fmt.Errorf("GetProcessTimes: %w", callErr)
	}

	return processSnapshot{
		status:              "supported",
		workingSetBytes:     metricPointer(uint64(counters.WorkingSetSize)),
		peakWorkingSetBytes: metricPointer(uint64(counters.PeakWorkingSetSize)),
		userCPUNS:           metricPointer(filetimeDurationNS(userTime)),
		systemCPUNS:         metricPointer(filetimeDurationNS(kernelTime)),
	}, nil
}

func (r *windowsProcessMetricReader) Close() error {
	return syscall.CloseHandle(r.handle)
}

func filetimeDurationNS(value syscall.Filetime) uint64 {
	ticks100NS := uint64(value.HighDateTime)<<32 | uint64(value.LowDateTime)
	return ticks100NS * 100
}
