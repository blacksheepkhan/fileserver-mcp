package benchmark

import "sort"

// MetricSummary describes a distribution of integer measurements.
type MetricSummary struct {
	Samples int    `json:"samples"`
	Min     uint64 `json:"min"`
	P50     uint64 `json:"p50"`
	P95     uint64 `json:"p95"`
	Max     uint64 `json:"max"`
}

func summarize(values []uint64) MetricSummary {
	if len(values) == 0 {
		return MetricSummary{}
	}

	sorted := append([]uint64(nil), values...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })

	return MetricSummary{
		Samples: len(sorted),
		Min:     sorted[0],
		P50:     percentile(sorted, 50),
		P95:     percentile(sorted, 95),
		Max:     sorted[len(sorted)-1],
	}
}

func percentile(sorted []uint64, percent int) uint64 {
	index := (len(sorted)*percent + 99) / 100
	if index < 1 {
		index = 1
	}
	return sorted[index-1]
}
