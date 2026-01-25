# Performance Audit

## Scope

- Focus on `ii todo` flows and the `todo` package first.
- Keep benchmark data, profiling commands, and notable improvements here.

## Benchmark setup

- Command (JSONL read/write): `go test ./todo -bench='(ReadJSONLFromReader|WriteJSONL)' -run=^$ -benchmem`
- Command (store operations): `go test ./todo -bench='Store(List|Ready)' -run=^$ -benchmem`
- Environment: darwin/arm64 (Apple M1 Ultra)
- Benchmark data: JSONL payload synthesized in-memory by `BenchmarkReadJSONLFromReader*`.

## Measurements

2026-01-25

| Benchmark | ns/op | B/op | allocs/op |
| --- | --- | --- | --- |
| `BenchmarkReadJSONLFromReader1K` | 1,719,032 | 734,003 | 10,003 |
| `BenchmarkReadJSONLFromReader10K` | 17,422,132 | 6,813,648 | 100,003 |
| `BenchmarkWriteJSONL1K` | 932,702 | 355,044 | 3,010 |
| `BenchmarkWriteJSONL10K` | 8,086,913 | 2,952,787 | 30,019 |
| `BenchmarkStoreList1K` | 1,799,685 | 923,378 | 10,013 |
| `BenchmarkStoreList10K` | 18,601,249 | 8,657,735 | 100,013 |
| `BenchmarkStoreReady1K` | 2,082,861 | 1,172,530 | 12,036 |
| `BenchmarkStoreReady10K` | 20,747,825 | 10,339,626 | 120,070 |
| `BenchmarkStoreReadyLimit10K` | 20,741,943 | 10,343,639 | 120,082 |

## Improvements log

- 2026-01-25: Replaced streaming JSONL decoding with a buffered line reader to preserve one-object-per-line semantics and enforce the max JSON line size guard deterministically.
- 2026-01-25: Avoided copying when a JSONL line fits in the buffered reader by returning the underlying slice, reducing allocations per line.
- 2026-01-25: Buffered JSONL writes before renaming the temp file, trimming syscall overhead and improving write throughput.
- 2026-01-25: Added benchmarks for store-level list/ready operations to track end-to-end todo command costs beyond JSONL parsing.
- 2026-01-25: Estimated JSONL item counts from reader sizes to preallocate slices, cutting bytes/op for reads and store queries.
- 2026-01-25: Preallocated list/ready result slices and blocker maps so store queries avoid repeated growth allocations when scanning todos.
- 2026-01-25: Added a ready-limit benchmark and switched ready queries with limits to a heap selection pass before sorting, keeping ranking consistent while avoiding full sorts for small limits.
- 2026-01-25: Increased JSONL read/write buffer sizes to 64 KiB to reduce syscall overhead during line reads and batched writes.

## Profiling notes

- 2026-01-25 (BenchmarkReadJSONLFromReader10K): CPU profile dominated by runtime.madvise and scheduler/GC work, suggesting allocations are the primary cost driver.
- 2026-01-25 (BenchmarkReadJSONLFromReader10K): Heap profile allocations concentrate in readJSONLFromReader/readJSONLLine buffering and encoding/json.Unmarshal.
- 2026-01-25 (BenchmarkStoreList10K): CPU profile shows syscall/syscall and bufio.Reader.ReadSlice at the top, indicating file I/O and buffering overhead dominate list queries.
- 2026-01-25 (BenchmarkStoreList10K): Heap profile is mostly readJSONLFromReader and encoding/json.Unmarshal allocations while loading todos, with list filtering contributing minimal alloc space.
- 2026-01-25 (BenchmarkStoreReady10K): CPU profile again centers on syscall overhead and buffered reads, matching list workloads.
- 2026-01-25 (BenchmarkStoreReady10K): Heap allocations come from readJSONLFromReader for todos/dependencies plus JSON decoding and building the ID map.
- 2026-01-25 (BenchmarkWriteJSONL10K): CPU profile dominated by syscall.syscall, indicating buffered writes still spend most time in file I/O.
- 2026-01-25 (BenchmarkWriteJSONL10K): Heap allocations concentrate in writeJSONL and time.Time.MarshalJSON via encoding/json, showing time encoding as the largest allocation source.

## Profiling commands

- CPU profile: `go test ./todo -bench=ReadJSONLFromReader10K -run=^$ -benchmem -cpuprofile /tmp/ii-todo-read-jsonl.pprof`
- Heap profile: `go test ./todo -bench=ReadJSONLFromReader10K -run=^$ -benchmem -memprofile /tmp/ii-todo-read-jsonl.mem.pprof`
- CPU profile (store list): `go test ./todo -bench=StoreList10K -run=^$ -benchmem -cpuprofile /tmp/ii-todo-store-list.pprof`
- Heap profile (store list): `go test ./todo -bench=StoreList10K -run=^$ -benchmem -memprofile /tmp/ii-todo-store-list.mem.pprof`
- CPU profile (store ready): `go test ./todo -bench=StoreReady10K -run=^$ -benchmem -cpuprofile /tmp/ii-todo-store-ready.pprof`
- Heap profile (store ready): `go test ./todo -bench=StoreReady10K -run=^$ -benchmem -memprofile /tmp/ii-todo-store-ready.mem.pprof`
- CPU profile (write JSONL): `go test ./todo -bench=WriteJSONL10K -run=^$ -benchmem -cpuprofile /tmp/ii-todo-write-jsonl.pprof`
- Heap profile (write JSONL): `go test ./todo -bench=WriteJSONL10K -run=^$ -benchmem -memprofile /tmp/ii-todo-write-jsonl.mem.pprof`
- Explore: `go tool pprof -http :0 /tmp/ii-todo-read-jsonl.pprof`
