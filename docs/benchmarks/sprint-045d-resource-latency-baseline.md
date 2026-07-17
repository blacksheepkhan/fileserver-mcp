# Sprint 3.45d resource, latency, and payload baseline

Date: 2026-07-15

## Scope

Sprint 3.45d implements BL-189 through BL-199 without changing public tool names, parameters, successful results, or tool-error behavior. The existing six-fixture serialization benchmark remains the source for historical direct, text-only, and text-plus-structured comparisons.

The new implementation adds direct in-process handler measurement, deterministic wire-size gates, a real STDIO process runner, ten reference workflows, machine-readable result schema/baseline/budgets, and Windows/Linux resource collectors using only the Go standard library.

## Measurement architecture

### In-process

`BenchmarkCallToolResultSerialization` still reports `ns/op`, `B/op`, `allocs/op`, and `payload-bytes` for all existing variants and now also reports full JSON-RPC `response-bytes`. `BenchmarkCallToolHandlerProcessing` measures validation, tool execution against deterministic fakes, result construction, and wrapping. `BenchmarkToolsListWireSerialization` covers read-only and default profiles.

Tests pin all six result sizes and their response-envelope sizes. The existing `tools/list` test now pins these schema-bearing responses:

| Profile | Tools | Schemas | Request bytes | Result bytes | Response bytes |
|---|---:|---:|---:|---:|---:|
| Read-only | 3 | 3 | 59 | 2,099 | 2,134 |
| Default | 8 | 8 | 59 | 5,622 | 5,657 |

### End-to-end process

Each sample starts the real built server, sends `initialize`, validates the response, optionally performs the remaining workflow requests, samples resources while the process is alive, closes stdin, and requires controlled exit status zero.

The standard run uses one `first_process_start` sample and 30 `subsequent_process_start` samples. Quick mode uses one first sample and 10 subsequent samples. `first_process_start` means first process after the script's build step; it is not a claim that an OS cold cache was forced.

### Reference workflows

The ten workflows are versioned in `benchmarks/workflows.json`. Multiple-operation workflows use ten independent calls, so their MCP call-count benefit can be compared with future batch operations without conflating MCP exchanges with model turns.

Filesystem byte and entry counters are derived from successful structured results and the known operation semantics. They remain private to the benchmark runner.

## Windows quick baseline

The versioned `benchmarks/baseline.windows-amd64.json` was produced from the uncommitted Sprint working tree based on commit `51e97b85d73726c6b4ea9c2898efbbb94e87a7f0` using Go `go1.26.4`, one first start, 10 subsequent starts, and 10 workflow repetitions. It records `working_tree_dirty: true` so the base commit is not misrepresented as the complete source state. It is a local review baseline, not a universal performance promise.

The run successfully reported Win32 resource metrics, zero server stderr warnings, zero hard budget failures, and zero unsupported metrics. Exact timing and resource distributions remain in the JSON artifact because they are machine- and load-sensitive.

Deterministic workflow maxima in that baseline are:

| Workflow | Calls | Request bytes | Response bytes | Result bytes | Read bytes | Entries |
|---|---:|---:|---:|---:|---:|---:|
| initialize | 0 | 166 | 151 | 116 | 0 | 0 |
| initialize → tools/list | 0 | 225 | 2,285 | 2,215 | 0 | 0 |
| get_path_info existing | 1 | 283 | 426 | 356 | 0 | 0 |
| get_path_info missing | 1 | 282 | 326 | 256 | 0 | 0 |
| read_file small | 1 | 276 | 355 | 285 | 26 | 0 |
| read_file 64 KiB | 1 | 281 | 131,378 | 131,308 | 65,536 | 0 |
| list_directory small | 1 | 281 | 538 | 468 | 0 | 3 |
| list_directory 500 | 1 | 281 | 53,680 | 53,610 | 0 | 500 |
| multiple path checks | 10 | 1,458 | 3,123 | 2,736 | 0 | 0 |
| multiple file reads | 10 | 1,408 | 2,193 | 1,806 | 260 | 0 |

The 64-KiB result remains approximately doubled by the selected text-plus-structured MCP contract, consistent with the Sprint 3.45a serialization baseline.

## Platform differences

Windows measurements use current and peak working set plus user/kernel process time from Win32. Linux uses `VmRSS`, `VmHWM`, and procfs user/system CPU ticks. Linux source compatibility is validated by cross-building the benchmark command; a native Linux run should record its own `baseline.linux-<arch>.json` rather than reuse Windows soft measurements.

Other platforms explicitly use `not_supported` and omit resource numbers. They do not serialize zero placeholders as if measurements succeeded.

## Budgets

Hard budgets equal the deterministic wire/counter baseline. Selected text-plus-structured allocation records retain the established six allocations per operation. Timing and resource budgets are intentionally broad soft review limits to avoid normal runner noise failing builds.

The local runner evaluates budgets now. Adding full benchmark execution and baseline comparison to CI is intentionally deferred to BL-247 and BL-248.

## Token orientation

The only token field is `approx_tokens_bytes4 = ceil(UTF-8 bytes / 4)`. It is approximate, not model-specific, uses no tokenizer library, and is unsuitable for billing.

## Security and release boundary

Results exclude binary paths, corpus roots, user names, raw environment variables, and secrets. Known local paths are replaced in captured stderr before serialization. The corpus is deleted after the run.

`cmd/benchmark` is not added to release workflows. Release artifacts remain server binaries built from `cmd/server`.
