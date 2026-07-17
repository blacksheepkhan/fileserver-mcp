# FlashGate MCP benchmarks

Sprint 3.45d provides one reproducible benchmark system with three layers. It extends the existing `tools/call` serialization fixtures instead of creating a competing serialization suite.

## Layers

1. In-process Go benchmarks measure direct `tools/call` handler work, result construction, JSON serialization, allocations, result bytes, response bytes, and `tools/list` wire output for read-only and default profiles.
2. `cmd/benchmark` starts the real previously built FlashGate binary and exchanges JSON-RPC over STDIO.
3. Ten reference workflows use a deterministic temporary corpus and report calls, wire sizes, result sizes, duration, filesystem counters, entry counts, resources, and the optional byte-based token approximation.

`cmd/benchmark` is a development tool. Release workflows continue to build and publish only `cmd/server`.

## Commands

Windows standard run (one first process plus 30 subsequent processes and 30 repetitions per workflow):

```powershell
& ".\scripts\benchmark.ps1"
```

Windows quick run (one first process plus 10 subsequent processes and 10 repetitions per workflow):

```powershell
& ".\scripts\benchmark.ps1" -Quick
```

Record an explicitly reviewable Windows baseline:

```powershell
& ".\scripts\benchmark.ps1" -Quick -RecordBaseline
```

Linux equivalents:

```bash
bash scripts/benchmark.sh
bash scripts/benchmark.sh --quick
bash scripts/benchmark.sh --quick --record-baseline
```

Run only the in-process benchmarks:

```bash
go test -run '^$' -bench 'Benchmark(CallToolResultSerialization|CallToolHandlerProcessing.*|ToolsListWireSerialization)$' -benchmem ./internal/mcp/tools ./cmd/server
```

## Start and resource semantics

`first_process_start` is the first new server process started by the benchmark immediately after the scripts build both binaries. `subsequent_process_start` contains later new processes in the same run. These labels do not claim that an operating-system cold filesystem or executable cache was guaranteed or cleared.

Startup duration begins immediately before the operating-system process start call and ends when a valid `initialize` response has been received. Workflow duration begins at the same point and ends when the final valid response for the workflow has been received. Controlled stdin closure and process exit validation happen after the measured response interval.

On Windows, the runner uses `OpenProcess`, `GetProcessMemoryInfo`, and `GetProcessTimes` to read current/peak working set plus user/kernel CPU time. On Linux, it reads `VmRSS` and `VmHWM` from `/proc/<pid>/status` and user/system CPU ticks from `/proc/<pid>/stat`. Other operating systems report resource status `not_supported`, omit unsupported numeric metrics, and list the metric names in `unsupported_metrics`; they never emit plausible zero placeholders.

Idle working set is sampled immediately after `initialize`. Peak working set and CPU time are sampled after the final workflow response while the server is still alive.

## Counter definitions

- `request_bytes`: complete UTF-8 JSON-RPC request including its JSONL newline.
- `response_bytes`: complete UTF-8 JSON-RPC response including its JSONL newline.
- `result_bytes`: only the serialized JSON value in the JSON-RPC `result` member.
- `read_bytes`: content bytes successfully returned by `read_file`.
- `written_bytes`: content bytes successfully written or copied; all Sprint 3.45d read-only reference workflows correctly report zero.
- `scanned_bytes`: content bytes actually inspected; for the current read workflows this equals successfully read content bytes. Metadata and directory enumeration do not scan file content.
- `entries`: directory entries actually returned by successful reference calls.
- `calls`: `tools/call` requests actually executed successfully. `initialize` and `tools/list` are not counted.

These benchmark counters are runner-side measurements only. Sprint 3.45d does not add them to public MCP tool results.

All byte counts include the initialization exchange when initialization is part of a named workflow. The separate `tools_list_measurements` entries contain only the `tools/list` request and response.

## Reference workflows

The machine-readable catalog is `workflows.json`. It covers initialize, initialize plus `tools/list`, existing and missing `get_path_info`, small and 64-KiB reads, small and 500-entry directory listings, ten independent path checks, and ten independent file reads. Every repetition starts a new read-only server process.

The corpus is created below the operating-system temporary directory, is removed after the run, and is never serialized into results.

## Token approximation

`approx_tokens_bytes4` is exactly:

```text
ceil(utf8_bytes / 4)
```

It is an orientation only, is not model-specific, does not use a tokenizer, and is not suitable for billing. Workflow values approximate complete response bytes; `tools/list` values approximate its complete response bytes.

## Baselines and budgets

`baseline.schema.json` defines result format `flashgate-benchmark/v1`. A result records project, commit, whether the binary came from a dirty working tree, Go version, OS, architecture, repetitions, starts, resources, `tools/list`, workflows, warnings, budget evaluation, and unsupported metrics.

Versioned results must not contain absolute host paths, user names, secrets, temporary directory names, or raw private environment variables. The runner never serializes its binary path, corpus root, or environment and replaces known paths in captured stderr.

`budgets.json` separates deterministic hard contracts from noisy soft review limits:

- Hard: tool/schema counts, wire/result byte maxima, reference workflow calls/counters, and stable selected-result allocation/payload records.
- Soft: startup p95, workflow p95, idle/peak working set, and CPU time.

A hard failure makes the local benchmark command fail after writing its JSON result. A soft excess is recorded as a warning for review. Sprint 3.45d does not add the full process benchmark to CI; cross-run baseline comparison and CI enforcement remain BL-247 and BL-248.

## Version 1.0 benchmark expansion

The existing Sprint 3.45d baseline remains a historical current-implementation baseline. It is not retroactively rewritten when Version 1.0 contracts change.

Version 1.0 extends the benchmark system with the following measurements:

- useful payload bytes distinct from response and result bytes;
- wire amplification factor: `response_bytes / useful_payload_bytes` for payload-bearing operations;
- approximate token cost per useful byte;
- initialization instructions bytes and approximate tokens;
- per-profile `tools/list` bytes, approximate tokens, tool count, and deterministic catalog fingerprint;
- direct STDIO, proxy-to-service, and service-backend latency/CPU/memory overhead;
- per-principal queue, concurrency, fairness, and overload behavior;
- large-result inline, page, cursor, stream, and resource-handle behavior;
- text, media, binary, directory, search, process-output, and system-information payload classes;
- audit/logging overhead under bounded normal load;
- native adapter versus any proposed external native-program adapter before adoption.

Payload-heavy content must be counted once as useful payload even when a client-compatibility fallback causes additional wire bytes. Metadata-only operations report zero useful payload and are evaluated through absolute response/catalog budgets rather than division by zero.

## Runtime-mode benchmark matrix

Version 1.0 records at least:

| Mode | Required measurements |
|---|---|
| Direct STDIO | startup, initialize, catalog, workflow latency, CPU, RSS/working set, wire/result/useful bytes |
| Proxy plus local service | proxy startup, connection/handshake, end-to-end latency, proxy/service CPU and memory, IPC bytes |
| System service backend | idle service cost, per-client cost, concurrency, queueing, caller authorization, service-account backend cost |
| Auto mode | successful service discovery and no-service direct fallback without elevation |

Windows and Linux results remain separate. A platform result must not substitute unsupported metrics with plausible zero values.

## Cross-project comparison

`BL-259` adds a reproducible comparison against pinned versions or commits of:

1. FlashGate MCP;
2. the official Node.js filesystem reference server;
3. one selected native Rust filesystem MCP;
4. one selected Go filesystem MCP.

The comparison uses the same host, corpus, requested functionality, warm-up policy, repetitions, payload definitions, and reporting format. Missing functionality is reported as `not_supported`; it is not emulated through unmeasured wrappers. The report must archive exact versions, configurations, commands, and raw machine-readable results and must not claim superiority outside the measured workflows.

## Version 1.0 release use

The Version 1.0 gate in `BL-261` requires approved hard budgets for deterministic protocol/catalog/payload contracts and reviewed soft budgets for host-sensitive latency, CPU, and memory. New optional accelerators or external programs are not accepted into the initial release unless they demonstrate a material benefit and pass the same security and portability review.
