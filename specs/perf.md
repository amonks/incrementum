# Performance Audit

## Scope

- Focus on `ii todo` flows and the `todo` package first.
- Keep benchmark data, profiling commands, and notable improvements here.

## Benchmark setup

- Command (JSONL read/write): `go test ./todo -bench='(ReadJSONLFromReader|WriteJSONL)' -run=^$ -benchmem`
- Command (store operations): `go test ./todo -bench='Store(DepTree|List|Ready)' -run=^$ -benchmem`
- Environment: darwin/arm64 (Apple M1 Ultra)
- Benchmark data: JSONL payload synthesized in-memory by `BenchmarkReadJSONLFromReader*`.

## Measurements

2026-01-25

| Benchmark | ns/op | B/op | allocs/op |
| --- | --- | --- | --- |
| `BenchmarkReadJSONLFromReader1K` | 1,728,882 | 477,587 | 9,004 |
| `BenchmarkReadJSONLFromReader10K` | 17,345,338 | 4,835,432 | 90,008 |
| `BenchmarkWriteJSONL1K` | 463,805 | 66,984 | 11 |
| `BenchmarkWriteJSONL10K` | 2,958,370 | 66,984 | 11 |
| `BenchmarkStoreList1K` | 1,791,515 | 666,805 | 9,011 |
| `BenchmarkStoreList10K` | 18,271,711 | 6,679,561 | 90,015 |
| `BenchmarkStoreReady1K` | 2,095,765 | 781,592 | 10,532 |
| `BenchmarkStoreReady10K` | 20,687,135 | 7,734,472 | 105,051 |
| `BenchmarkStoreReadyLimit10K` | 20,111,254 | 5,841,620 | 105,057 |
| `BenchmarkStoreDepTree1K` | 2,913,114 | 1,277,720 | 18,066 |
| `BenchmarkStoreDepTree10K` | 30,887,001 | 12,140,715 | 180,253 |

## Improvements log

- 2026-01-25: Reused the locked file descriptor for JSONL reads to avoid extra open/close syscalls during store reads.
- 2026-01-25: Replaced streaming JSONL decoding with a buffered line reader to preserve one-object-per-line semantics and enforce the max JSON line size guard deterministically.
- 2026-01-25: Avoided copying when a JSONL line fits in the buffered reader by returning the underlying slice, reducing allocations per line.
- 2026-01-25: Buffered JSONL writes before renaming the temp file, trimming syscall overhead and improving write throughput.
- 2026-01-25: Added benchmarks for store-level list/ready operations to track end-to-end todo command costs beyond JSONL parsing.
- 2026-01-25: Estimated JSONL item counts from reader sizes to preallocate slices, cutting bytes/op for reads and store queries.
- 2026-01-25: Preallocated list/ready result slices and blocker maps so store queries avoid repeated growth allocations when scanning todos.
- 2026-01-25: Added a ready-limit benchmark and switched ready queries with limits to a heap selection pass before sorting, keeping ranking consistent while avoiding full sorts for small limits.
- 2026-01-25: Increased JSONL read/write buffer sizes to 64 KiB to reduce syscall overhead during line reads and batched writes.
- 2026-01-25: When Ready runs with a limit, select into the heap while scanning instead of building the full ready slice, trimming bytes/op for ready-limit benchmarks.
- 2026-01-25: Disabled HTML escaping in JSONL writes to shave overhead from JSON encoding while preserving valid output.
- 2026-01-25: Resolve dependency blocker statuses by the dependency ID set instead of mapping every todo, cutting Ready bytes/op by avoiding a full todo lookup map.
- 2026-01-25: Switched JSONL line reads to bufio.ReadLine to avoid extra newline handling work while keeping the max line size guard intact.
- 2026-01-25: Dropped redundant JSONL line-ending trimming now that bufio.ReadLine already strips terminators, reducing per-line work during JSONL reads.
- 2026-01-25: Unmarshal JSONL data directly into the destination slice slot to avoid copying each decoded item, trimming allocations across read and store benchmarks.
- 2026-01-25: Added custom JSONL encoders for todos and dependencies to avoid per-field time.Time MarshalJSON allocations during writes.
- 2026-01-25: Reused scratch buffers while encoding JSONL writes so each item avoids fresh allocations, cutting write allocations and improving throughput.
- 2026-01-25: Reused the todo list/ready in-memory results to compute ID prefix lengths in `ii todo`, eliminating redundant JSONL reads for list/ready output.
- 2026-01-25: Added dependency tree benchmarks to track `ii todo dep tree` performance at scale.
- 2026-01-25: Reused the JSONL line buffer when assembling oversized lines so multi-chunk reads avoid repeated allocations.
- 2026-01-25: Preallocated dependency maps and per-node children slices when building dep trees to reduce allocation churn during dep tree queries.
- 2026-01-25: Pooled JSONL reader buffers to reuse the 64 KiB read and line scratch buffers across reads, reducing read allocations and downstream store bytes/op.

## Profiling notes

- 2026-01-25 (BenchmarkReadJSONLFromReader10K): CPU profile dominated by runtime.madvise and scheduler/GC work, suggesting allocations are the primary cost driver.
- 2026-01-25 (BenchmarkReadJSONLFromReader10K): Heap profile allocations concentrate in readJSONLFromReader/readJSONLLine buffering and encoding/json.Unmarshal.
- 2026-01-25 (BenchmarkStoreList10K): CPU profile shows syscall/syscall and bufio.Reader.ReadSlice at the top, indicating file I/O and buffering overhead dominate list queries.
- 2026-01-25 (BenchmarkStoreList10K): Heap profile is mostly readJSONLFromReader and encoding/json.Unmarshal allocations while loading todos, with list filtering contributing minimal alloc space.
- 2026-01-25 (BenchmarkStoreReady10K): CPU profile again centers on syscall overhead and buffered reads, matching list workloads.
- 2026-01-25 (BenchmarkStoreReady10K): Heap allocations come from readJSONLFromReader for todos/dependencies plus JSON decoding, with dependency blocker maps contributing the next largest share.
- 2026-01-25 (BenchmarkStoreReadyLimit10K): CPU profile still dominated by syscall/syscall with json decoding work next, so Ready-limit performance remains bound by file I/O.
- 2026-01-25 (BenchmarkStoreReadyLimit10K): Heap allocations mostly come from readJSONLFromReader and encoding/json.Unmarshal, with dependency blocker maps contributing the next largest share.
- 2026-01-25 (BenchmarkWriteJSONL10K): CPU profile dominated by syscall.syscall while appendTodoJSONLine and bufio.Writer.Write account for the remaining on-CPU time, confirming file I/O remains the hot path.
- 2026-01-25 (BenchmarkWriteJSONL10K): Heap allocations mainly come from the buffered writer setup and benchmark data generation, with no per-item JSON encoding allocations showing up in the profile.
- 2026-01-25 (BenchmarkStoreDepTree10K): CPU profile still dominated by syscall.syscall and runtime.madvise, with JSON decoding work next, so dependency tree queries remain file I/O bound.
- 2026-01-25 (BenchmarkStoreDepTree10K): Heap allocations are led by JSONL reads and encoding/json.Unmarshal, with buildDepTree and ID normalization contributing the next largest shares.

## Profiling commands

- CPU profile: `go test ./todo -bench=ReadJSONLFromReader10K -run=^$ -benchmem -cpuprofile /tmp/ii-todo-read-jsonl.pprof`
- Heap profile: `go test ./todo -bench=ReadJSONLFromReader10K -run=^$ -benchmem -memprofile /tmp/ii-todo-read-jsonl.mem.pprof`
- CPU profile (store list): `go test ./todo -bench=StoreList10K -run=^$ -benchmem -cpuprofile /tmp/ii-todo-store-list.pprof`
- Heap profile (store list): `go test ./todo -bench=StoreList10K -run=^$ -benchmem -memprofile /tmp/ii-todo-store-list.mem.pprof`
- CPU profile (store ready): `go test ./todo -bench=StoreReady10K -run=^$ -benchmem -cpuprofile /tmp/ii-todo-store-ready.pprof`
- Heap profile (store ready): `go test ./todo -bench=StoreReady10K -run=^$ -benchmem -memprofile /tmp/ii-todo-store-ready.mem.pprof`
- CPU profile (store ready limit): `go test ./todo -bench=StoreReadyLimit10K -run=^$ -benchmem -cpuprofile /tmp/ii-todo-store-ready-limit.pprof`
- Heap profile (store ready limit): `go test ./todo -bench=StoreReadyLimit10K -run=^$ -benchmem -memprofile /tmp/ii-todo-store-ready-limit.mem.pprof`
- CPU profile (write JSONL): `go test ./todo -bench=WriteJSONL10K -run=^$ -benchmem -cpuprofile /tmp/ii-todo-write-jsonl.pprof`
- Heap profile (write JSONL): `go test ./todo -bench=WriteJSONL10K -run=^$ -benchmem -memprofile /tmp/ii-todo-write-jsonl.mem.pprof`
- CPU profile (store dep tree): `go test ./todo -bench=StoreDepTree10K -run=^$ -benchmem -cpuprofile /tmp/ii-todo-store-deptree.pprof`
- Heap profile (store dep tree): `go test ./todo -bench=StoreDepTree10K -run=^$ -benchmem -memprofile /tmp/ii-todo-store-deptree.mem.pprof`
- Explore: `go tool pprof -http :0 /tmp/ii-todo-read-jsonl.pprof`
