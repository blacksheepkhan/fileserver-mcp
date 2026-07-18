# Coding and Architecture Style

## Go baseline

- Use the repository Go version and standard formatting.
- Prefer the standard library and small explicit abstractions.
- Keep packages cohesive and dependencies directed inward toward domain contracts.
- Avoid global mutable state; construct dependencies explicitly.
- Return errors rather than logging inside domain packages.
- Preserve cancellation and deadlines through `context.Context` at operation boundaries.
- Bound goroutines, queues, buffers, recursion, file descriptors, processes, and retained results.

## Layering

- MCP/JSON-RPC DTOs belong in protocol/adapter packages.
- Domain services expose protocol-independent request/result types.
- Security, identity, policy, quotas, audit, and execution-backend selection are explicit dependencies.
- Platform-specific code is isolated behind narrow interfaces and build-tagged files where appropriate.
- Filesystem and process tools do not call OS APIs directly when an owning adapter/service exists.
- Service/proxy transport does not contain domain logic.

## Native implementation

Use, in order:

1. Go standard library;
2. platform-specific Go implementation;
3. direct stable OS API or system interface;
4. reviewed allowlisted external native program.

Do not add an interpreter runtime to implement an MCP operation. Deployment scripts may use `pwsh` on Windows or the system shell on Linux, but scripts are not hidden runtime dependencies and must be documented, bounded, and non-secret-bearing.

## Errors and diagnostics

- Classify errors internally and map them at the adapter boundary.
- Do not return absolute host paths, raw command lines, credentials, environment dumps, tokens, stack traces, or unsafe OS error strings.
- Keep stdout reserved for protocol output in STDIO/proxy modes.
- Use structured audit/diagnostic fields and sanitize untrusted text against log injection.
- Preserve a stable correlation ID without exposing internal security identifiers unnecessarily.

## Concurrency and lifecycle

- Every goroutine has an owner and termination path.
- Every handle has owner principal, root/profile/backend context, TTL, and cleanup behavior.
- Per-principal and global limits are checked before resource allocation.
- Slow consumers and blocked subprocesses cannot grow memory without bound.
- Process trees, temporary files, result resources, sockets/pipes, and jobs are cleaned deterministically on cancellation, expiry, crash recovery, and shutdown.

## Data and payloads

- Use typed structs for stable productive results.
- Keep ordering deterministic.
- Avoid duplicating payload-heavy data between text, structured, audit, cache, and IPC representations.
- Page or stream large collections and content; do not read whole files or process output without explicit bounded need.
- Copy byte buffers only when ownership or safety requires it.
- Never expose an absolute host path in a public result/resource identifier.

## Commands

- Never construct a shell command string from client input.
- Resolve server-defined command IDs to reviewed executable definitions.
- Build direct argument vectors from closed typed values.
- Restrict environment, working directory, paths, hooks, plugins, response files, config injection, network behavior, time, output, and process tree.
- Add platform-specific negative tests for quoting and executable resolution.

## Tests

- Unit-test pure domain and policy logic.
- Contract-test schemas, ordering, annotations, result classes, and error mapping.
- Integration-test real Windows/Linux path, process, service, IPC, and identity behavior.
- Add regression tests for every security bug.
- Benchmark material changes and record deterministic payload/catalog budgets.
- Use fakes only where they do not conceal required OS semantics.
