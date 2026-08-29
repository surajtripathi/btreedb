# btreedb

A disk-based B-tree key-value store, built from scratch in Go — slotted-page
layout, write-ahead log, threshold checkpointing, leaf-chained range scans,
and delete. Built step by step (see [`btree_roadmap.html`](btree_roadmap.html))
and benchmarked head-to-head against an LSM-tree sibling project (see
[`BtreeVsLSM.md`](BtreeVsLSM.md)).

This is a learning/systems project: the goal was to understand *why*
B-trees and LSM-trees make the tradeoffs they do by implementing one for
real, not just reading about it.

## Features

- **Fixed 4KB slotted pages** — header + slot directory growing forward,
  cell data growing backward, for leaf, internal, and superblock node types.
- **Real multi-level B-tree** — recursive leaf/internal splits, propagated
  up to and including creating a new root, verified to 4+ levels under
  30k+ inserts.
- **Write-ahead log** — every `Put`/`Delete` is appended (with CRC32
  checksums to detect torn writes) and synced before being applied to the
  tree, with correct crash-replay semantics on `Open`.
- **Threshold-based checkpointing** — the pager is synced and the WAL
  truncated every N operations rather than on every single write; a clean
  `Close()` always forces a final checkpoint.
- **Range scans** — `RangeGet` descends once via the shared root-to-leaf
  path, then walks sideways across the leaf chain (`nextPageID`), so
  per-entry cost amortizes as the range grows.
- **Delete** — no underflow handling (merge/borrow) yet; deferred
  deliberately, the same way Postgres lazily defers reclamation to
  `VACUUM` rather than eager rebalancing.

## Layout

| File | Responsibility |
|---|---|
| `pager.go` | Raw page I/O against a file — read/write/allocate by page ID. Knows nothing about keys or trees. |
| `leaf_node.go` | Leaf page encode/decode, search, insert, split, delete. |
| `intermediate_node.go` | Internal (non-leaf) page encode/decode, child search, insert, split. |
| `superblock_node.go` | Page 0 — persists the current root page ID across restarts. |
| `wal.go` | Write-ahead log: append, replay, truncate, with CRC32-checked records. |
| `check_pointing.go` | Counter/threshold tracking for when to checkpoint. |
| `store.go` | Ties it all together: `Open`, `Get`, `Put`, `Delete`, `RangeGet`, split/checkpoint orchestration. |
| `cmd/kvcli` | A tiny REPL for poking at the store by hand (`put`/`get`/`del`), useful for manually reproducing crash/replay scenarios. |

## Usage

```go
import btreestore "btreedb"

store, err := btreestore.Open(btreestore.StoreOptions{
    DBPath:                 "data.btree",
    WalPath:                "data.wal",
    CheckpointingThreshold: 1000, // sync/checkpoint every N ops
})
if err != nil {
    log.Fatal(err)
}
defer store.Close()

err = store.Put("foo", "bar")
val, err := store.Get("foo")
entries, err := store.RangeGet("a", "z")
err = store.Delete("foo")
```

Or try the CLI directly:

```sh
go run ./cmd/kvcli data.wal
> put foo bar
> get foo
```

The CLI's own comments walk through deliberately killing the process
mid-write to watch WAL replay discard a torn write safely.

## Testing & benchmarks

```sh
go test ./...                                    # correctness
go test -bench=. -benchmem -run=^$ -benchtime=100x  # benchmarks
```

Benchmark methodology notes (fixed iteration counts instead of
auto-scaled `b.N`, shared pre-populated stores for read comparisons, and
two artifacts this caught and fixed along the way) are documented at the
top of `store_benchmark_test.go` and expanded on in
[`BtreeVsLSM.md`](BtreeVsLSM.md).

## How it compares to an LSM-tree

`BtreeVsLSM.md` is a full written comparison against a companion LSM-tree
project (`kvstore`) implementing the same `Get`/`Put`/`Delete` interface.
Headline findings:

- **Get-miss** is ~18x faster on the LSM-tree — its bloom filter rules out
  a miss with a single hash check; btreedb has no early-exit and pays the
  same full descent whether the key exists or not.
- **Get-hit** favors the LSM-tree by ~2.2x, from its block index narrowing
  down which SSTable/block to read, versus btreedb's full page decode at
  every tree level.
- **Put** wall-clock time is close between the two, but allocations tell
  the real story: 6 allocs/op (LSM, near a single in-memory map insert) vs.
  256 allocs/op (btreedb, full page decode → mutate → re-encode on every
  call).
- **Range scans** are btreedb's clearest structural win — per-entry cost
  drops ~5.6x from a 10-entry to a 5,000-entry scan via sequential
  leaf-chain reads, something an LSM-tree's scattered-across-SSTables
  layout makes structurally harder.

The single highest-leverage next step the data points to: a bloom filter
in front of btreedb's read path (see below).

## Known limitations

Deferred deliberately, not overlooked — tracked in full detail in
`btree_roadmap.html`:

- **Split is not crash-atomic across page writes.** A crash between
  writing the split leaf's two halves and updating the parent (or
  superblock, on a root split) can leave the right half unreachable. Fix:
  never overwrite the original leaf until the entire split is fully
  durable.
- **No bloom filter / early-exit for misses** — the single highest-leverage
  change suggested by the benchmark comparison above.
- **A single key/value larger than ~half a page** can make a split still
  overflow. `encode()` fails cleanly (`ErrorOverFlow`) rather than
  corrupting data, but `Put` doesn't yet surface a clean "key too large"
  error.
- **Full page rebuild on every insert** rather than incremental in-place
  slot/cell edits. Correct and simple, but measurably the dominant
  allocation cost (256 allocs/op) — see `BtreeVsLSM.md` Finding 3 for the
  measured breakdown (allocation cost tracks entries-per-leaf, not tree
  depth).
- **No underflow handling on delete** — a leaf/internal node can shrink
  to near-empty without merging or borrowing from a sibling. Correct,
  just not space-efficient (same tradeoff Postgres makes with
  lazy-delete + `VACUUM`).

## Requirements

Go 1.26.5 (see `go.mod`).
