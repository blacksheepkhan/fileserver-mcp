//go:build !windows && !linux

package benchmark

type unsupportedProcessMetricReader struct{}

func newProcessMetricReader(_ int) (processMetricReader, error) {
	return unsupportedProcessMetricReader{}, nil
}

func (unsupportedProcessMetricReader) Snapshot() (processSnapshot, error) {
	return unsupportedSnapshot(), nil
}

func (unsupportedProcessMetricReader) Close() error { return nil }
