package benchmark

import "testing"

func TestSummarizeUsesNearestRankPercentiles(t *testing.T) {
	got := summarize([]uint64{100, 1, 4, 2, 3})
	want := MetricSummary{Samples: 5, Min: 1, P50: 3, P95: 100, Max: 100}
	if got != want {
		t.Fatalf("summarize()=%+v, want %+v", got, want)
	}
}

func TestApproxTokensBytes4RoundsUp(t *testing.T) {
	tests := map[uint64]uint64{0: 0, 1: 1, 4: 1, 5: 2, 8: 2}
	for input, want := range tests {
		if got := approxTokensBytes4(input); got != want {
			t.Fatalf("approxTokensBytes4(%d)=%d, want %d", input, got, want)
		}
	}
}

func TestReferenceWorkflowContract(t *testing.T) {
	wantCalls := map[string]int{
		"initialize":                 0,
		"initialize_tools_list":      0,
		"get_path_info_existing":     1,
		"get_path_info_missing":      1,
		"read_file_small":            1,
		"read_file_64kib":            1,
		"list_directory_small":       1,
		"list_directory_500_entries": 1,
		"multiple_path_checks":       10,
		"multiple_file_reads":        10,
	}
	workflows := referenceWorkflows()
	if len(workflows) != len(wantCalls) {
		t.Fatalf("workflow count=%d, want %d", len(workflows), len(wantCalls))
	}
	for _, workflow := range workflows {
		calls := 0
		for _, request := range workflow.requests {
			if request.Method == "tools/call" {
				calls++
			}
		}
		if calls != wantCalls[workflow.name] {
			t.Fatalf("workflow %s calls=%d, want %d", workflow.name, calls, wantCalls[workflow.name])
		}
	}
}
