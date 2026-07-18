package benchmark

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStrictArtifactValidatorRejectsSecurityRelevantMutations(t *testing.T) {
	benchmarkDirectory := filepath.Join("..", "..", "benchmarks")
	budgetPath := filepath.Join(benchmarkDirectory, "budgets.json")
	windowsSource := readTestArtifact(t, filepath.Join(benchmarkDirectory, "baseline.windows-amd64.json"))
	linuxSource := readTestArtifact(t, filepath.Join(benchmarkDirectory, "baseline.linux-amd64.json"))

	tests := []struct {
		name         string
		wantArtifact string
		wantCause    string
		mutate       func(*[]byte, *[]byte)
	}{
		{
			name: "hard budget exceeded with embedded zero evaluation", wantArtifact: "baseline.windows-amd64.json", wantCause: "budget failure",
			mutate: func(windows, _ *[]byte) {
				*windows = mutateResultArtifact(t, *windows, func(result *Result) {
					result.ToolsList[0].RequestBytes = 60
				})
			},
		},
		{
			name: "relevant value lowered with stale embedded evaluation", wantArtifact: "baseline.windows-amd64.json", wantCause: "budget_evaluation mismatch",
			mutate: func(windows, _ *[]byte) {
				*windows = mutateResultArtifact(t, *windows, func(result *Result) {
					original := result.StartMeasurements.SubsequentProcessStart.DurationNS
					result.StartMeasurements.SubsequentProcessStart.DurationNS.P95 = 100_000_001
					if result.StartMeasurements.SubsequentProcessStart.DurationNS.Max < 100_000_001 {
						result.StartMeasurements.SubsequentProcessStart.DurationNS.Max = 100_000_001
					}
					evaluation, err := EvaluateBudgets(budgetPath, *result)
					if err != nil {
						t.Fatal(err)
					}
					result.BudgetEvaluation = evaluation
					result.StartMeasurements.SubsequentProcessStart.DurationNS = original
				})
			},
		},
		{
			name: "embedded evaluation differs from recomputation", wantArtifact: "baseline.windows-amd64.json", wantCause: "budget_evaluation mismatch",
			mutate: func(windows, _ *[]byte) {
				*windows = mutateResultArtifact(t, *windows, func(result *Result) {
					result.BudgetEvaluation.HardFailures = 1
					result.BudgetEvaluation.Messages = []string{"hard: fabricated"}
				})
			},
		},
		{
			name: "only Windows artifact changed", wantArtifact: "baseline.windows-amd64.json", wantCause: "cross-platform deterministic projection",
			mutate: func(windows, _ *[]byte) {
				*windows = mutateResultArtifact(t, *windows, func(result *Result) {
					incrementConstantMetric(&result.Workflows[0].ApproxTokensBytes4)
				})
			},
		},
		{
			name: "only Linux artifact changed", wantArtifact: "baseline.linux-amd64.json", wantCause: "cross-platform deterministic projection",
			mutate: func(_, linux *[]byte) {
				*linux = mutateResultArtifact(t, *linux, func(result *Result) {
					incrementConstantMetric(&result.Workflows[0].ApproxTokensBytes4)
				})
			},
		},
		{
			name: "both platform artifacts identically exceed a hard budget", wantArtifact: "baseline.windows-amd64.json", wantCause: "budget failure",
			mutate: func(windows, linux *[]byte) {
				mutation := func(result *Result) { result.ToolsList[0].RequestBytes = 60 }
				*windows = mutateResultArtifact(t, *windows, mutation)
				*linux = mutateResultArtifact(t, *linux, mutation)
			},
		},
		{
			name: "unknown top-level field", wantArtifact: "baseline.windows-amd64.json", wantCause: "$.unexpected_top_level: unknown field",
			mutate: func(windows, _ *[]byte) {
				*windows = mutateJSONObject(t, *windows, func(object map[string]any) {
					object["unexpected_top_level"] = true
				})
			},
		},
		{
			name: "unknown nested measurement field", wantArtifact: "baseline.windows-amd64.json", wantCause: "unknown_nested: unknown field",
			mutate: func(windows, _ *[]byte) {
				*windows = mutateJSONObject(t, *windows, func(object map[string]any) {
					workflows := object["workflow_measurements"].([]any)
					workflow := workflows[0].(map[string]any)
					requestBytes := workflow["request_bytes"].(map[string]any)
					requestBytes["unknown_nested"] = 1
				})
			},
		},
		{
			name: "missing required field", wantArtifact: "baseline.windows-amd64.json", wantCause: "$.working_tree_dirty: missing required field",
			mutate: func(windows, _ *[]byte) {
				*windows = mutateJSONObject(t, *windows, func(object map[string]any) {
					delete(object, "working_tree_dirty")
				})
			},
		},
		{
			name: "wrong JSON type", wantArtifact: "baseline.windows-amd64.json", wantCause: "$.repetitions: expected number",
			mutate: func(windows, _ *[]byte) {
				*windows = mutateJSONObject(t, *windows, func(object map[string]any) {
					object["repetitions"] = "30"
				})
			},
		},
		{
			name: "null required value", wantArtifact: "baseline.windows-amd64.json", wantCause: "$.warnings: null is not allowed",
			mutate: func(windows, _ *[]byte) {
				*windows = mutateJSONObject(t, *windows, func(object map[string]any) {
					object["warnings"] = nil
				})
			},
		},
		{
			name: "trailing JSON object", wantArtifact: "baseline.windows-amd64.json", wantCause: "trailing JSON data",
			mutate: func(windows, _ *[]byte) {
				*windows = append(append([]byte{}, *windows...), []byte("\n{}")...)
			},
		},
		{
			name: "duplicate security-relevant property", wantArtifact: "baseline.windows-amd64.json", wantCause: "$.project: duplicate JSON field",
			mutate: func(windows, _ *[]byte) {
				*windows = append([]byte(`{"project":"flashgate-mcp",`), (*windows)[1:]...)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			windows := append([]byte{}, windowsSource...)
			linux := append([]byte{}, linuxSource...)
			tc.mutate(&windows, &linux)
			directory := t.TempDir()
			writeTestArtifact(t, filepath.Join(directory, "baseline.windows-amd64.json"), windows)
			writeTestArtifact(t, filepath.Join(directory, "baseline.linux-amd64.json"), linux)

			err := validatePlatformArtifactSet(directory, budgetPath)
			if err == nil || !strings.Contains(err.Error(), tc.wantArtifact) || !strings.Contains(err.Error(), tc.wantCause) {
				t.Fatalf("validator error=%v, want artifact %q and cause %q", err, tc.wantArtifact, tc.wantCause)
			}
		})
	}
}

func TestArtifactBudgetValidationPreservesSoftAndHardSemantics(t *testing.T) {
	benchmarkDirectory := filepath.Join("..", "..", "benchmarks")
	budgetPath := filepath.Join(benchmarkDirectory, "budgets.json")
	result, _, err := loadValidatedBaselineArtifact(filepath.Join(benchmarkDirectory, "baseline.windows-amd64.json"), budgetPath)
	if err != nil {
		t.Fatal(err)
	}

	soft := cloneBenchmarkResult(t, result)
	soft.StartMeasurements.SubsequentProcessStart.DurationNS.P95 = 100_000_001
	if soft.StartMeasurements.SubsequentProcessStart.DurationNS.Max < 100_000_001 {
		soft.StartMeasurements.SubsequentProcessStart.DurationNS.Max = 100_000_001
	}
	soft.BudgetEvaluation, err = EvaluateBudgets(budgetPath, soft)
	if err != nil {
		t.Fatal(err)
	}
	if soft.BudgetEvaluation.HardFailures != 0 || soft.BudgetEvaluation.SoftWarnings != 1 {
		t.Fatalf("soft fixture evaluation=%+v, want zero hard and one soft", soft.BudgetEvaluation)
	}
	if err := validateArtifactBudgets("soft-warning.json", budgetPath, soft); err != nil {
		t.Fatalf("matching soft warning must remain non-fatal: %v", err)
	}

	hard := cloneBenchmarkResult(t, result)
	hard.ToolsList[0].RequestBytes = 60
	hard.BudgetEvaluation, err = EvaluateBudgets(budgetPath, hard)
	if err != nil {
		t.Fatal(err)
	}
	if err := validateArtifactBudgets("hard-failure.json", budgetPath, hard); err == nil || !strings.Contains(err.Error(), "hard-failure.json budget failure") {
		t.Fatalf("matching embedded hard failure was not rejected: %v", err)
	}
}

func validatePlatformArtifactSet(directory string, budgetPath string) error {
	baselines, err := loadRequiredPlatformBaselines(directory, budgetPath)
	if err != nil {
		return err
	}
	for _, platform := range []string{"windows", "linux"} {
		if err := validateCompletePlatformBaseline(baselines[platform].result, platform); err != nil {
			return fmt.Errorf("artifact baseline.%s-amd64.json completeness: %w", platform, err)
		}
	}
	return validateDeterministicPlatformMatch(baselines["windows"].result, baselines["linux"].result)
}

func incrementConstantMetric(summary *MetricSummary) {
	summary.Min++
	summary.P50++
	summary.P95++
	summary.Max++
}

func mutateResultArtifact(t *testing.T, raw []byte, mutate func(*Result)) []byte {
	t.Helper()
	var result Result
	if err := decodeStrictJSON(raw, &result); err != nil {
		t.Fatal(err)
	}
	mutate(&result)
	encoded, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	return append(encoded, '\n')
}

func mutateJSONObject(t *testing.T, raw []byte, mutate func(map[string]any)) []byte {
	t.Helper()
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var object map[string]any
	if err := decoder.Decode(&object); err != nil {
		t.Fatal(err)
	}
	mutate(object)
	encoded, err := json.MarshalIndent(object, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	return append(encoded, '\n')
}

func readTestArtifact(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func writeTestArtifact(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}
