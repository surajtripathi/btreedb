# B-Tree vs. LSM-Tree: A Benchmark Comparison

Two key-value stores, built from scratch in Go, implementing the same
`Put` / `Get` / `Delete` interface over two fundamentally different storage
engines:

- **`btreedb`** — a disk-based B-tree, slotted-page layout, WAL + checkpointing,
  leaf-chained range scans.
- **`kvstore`** — an LSM-tree, memtable + WAL, block-indexed SSTables, k-way
  merge compaction, bloom filter.

This document reports the measured differences and explains the *why*
behind them, not just the numbers.

## Methodology

All comparisons use fixed iteration counts (`-benchtime=Nx`), never
auto-scaled `b.N`, when comparing two variants against each other.
Auto-scaling lets each variant run a different number of iterations, and
since operation cost in both engines depends on accumulated state (tree
depth, SSTable count), this alone can flip which variant looks faster — a
mistake caught directly during this work, where an early "checkpointing
makes Put slower" result turned out to be exactly this artifact rather
than a real effect.

Read benchmarks (`Get`, `RangeGet`) share a single pre-populated store via
`sync.Once`, so hit/miss/range comparisons run against an identical
dataset rather than separately built ones — another mistake caught along
the way: an initial ~20% Get-hit-vs-miss gap in `btreedb` turned out to be
an artifact of differently-shaped benchmark keys, not a real cost
difference, and disappeared once corrected.

## Results

| Operation | btreedb (B-tree) | kvstore (LSM-tree) |
|---|---|---|
| Put | 2,784,260 ns/op · 256 allocs/op | 2,707,336 ns/op · 6 allocs/op |
| Get (hit) | 6,383 ns/op · 241 allocs/op | 2,892 ns/op · 109 allocs/op |
| Get (miss) | 6,345 ns/op · 242 allocs/op | ~350 ns/op · 1 alloc/op |
| Delete | 2,546,730 ns/op · 149 allocs/op | *(not benchmarked)* |
| Range scan (10 keys) | 7,468 ns/op · 271 allocs/op | *(not benchmarked)* |
| Range scan (5,000 keys) | 664,315 ns/op · 10,487 allocs/op | *(not benchmarked)* |

*(darwin/amd64, VirtualApple @ 2.50GHz)*

## Finding 1: Get-miss is the sharpest architectural contrast

kvstore's Get-miss (~350 ns/op) is roughly **18x faster** than btreedb's
(~6,345 ns/op), and even faster than kvstore's own Get-hit. This is
entirely the bloom filter: a miss can be ruled out with a single hash
check and zero disk reads, before the block index or any SSTable is
touched.

btreedb has no equivalent. Every `Get` — hit or miss — pays the full
root-to-leaf descent, because nothing in the structure can cheaply prove
a key's absence short of walking to the leaf where it would live and
checking. Confirmed directly: btreedb's hit and miss costs are
statistically identical (6,383 vs 6,345 ns/op) once the benchmark
controlled for key shape, exactly as expected for a structure with no
early-exit mechanism.

This is a real, addressable gap rather than a fundamental limitation of
B-trees — a bloom filter is not LSM-specific, and could sit in front of a
B-tree's lookups too. It simply wasn't part of this build; a compelling
candidate for the "next thing to try" list.

## Finding 2: Get-hit favors the LSM-tree, more narrowly

kvstore's Get-hit (2,892 ns/op) beats btreedb's (6,383 ns/op) by roughly
2.2x, and allocates less (109 vs 241 allocs/op). This isn't the bloom
filter's doing (it only helps misses) — it reflects kvstore's block
index and min/max range filtering doing real, cheap work to narrow down
*which* SSTable and block to read, versus btreedb paying a full page
decode at every level of a root-to-leaf descent regardless of how deep
the tree is.

## Finding 3: Put — allocations tell the real story, not wall-clock time

Wall-clock Put cost is nearly identical (2.78ms vs 2.71ms) — not what
either engine's design would predict, since LSM-trees exist specifically
to make writes cheap by deferring expensive work to background
compaction. The allocation counts reveal what the timing doesn't: **6
allocs/op for kvstore vs. 256 for btreedb**, a ~43x difference.

This is the more trustworthy signal. kvstore's `Put`, at steady state, is
close to a single in-memory sorted-map insertion — genuinely
lightweight. btreedb's `Put` always pays a full page decode → mutate →
re-encode cycle, allocating a fresh 4KB buffer and rebuilding a slot
directory on every call, regardless of whether anything about the page's
size actually changed. The near-equal wall-clock numbers are most likely
an artifact of this specific benchmark's amortized flush/checkpoint
timing rather than a true reflection of each engine's steady-state
write cost — allocations/op is the number that actually reflects the
architectural difference.

This is also the most direct, measured argument for btreedb's own
deferred optimization (see `KNOWN_LIMITATIONS.md`): replacing full-page
rebuild with incremental in-place slot/cell edits. A follow-up
allocation-count investigation on btreedb's own Put (varying tree shape)
found that allocation cost is driven by **entries-per-leaf**, not tree
depth — a short-key, densely-packed tree (10,000 inserts, depth 1) showed
*more* allocs/op (301) than a long-key, much deeper tree at the same
insert count (depth 3, 97 allocs/op). The cost center is the leaf's
in-memory slice splice, not the number of levels touched — exactly where
an incremental-edit optimization would need to focus.

## Finding 4: Range scans amortize well — a real B-tree structural advantage

Per-entry cost in btreedb drops roughly 5.6x going from a 10-entry range
scan (~747 ns/entry) to a 5,000-entry scan (~133 ns/entry). The fixed
root-to-leaf descent cost is paid once; every subsequent entry is a
cheap sequential read across the `nextPageID` leaf chain — a design
decision made all the way back at the page-format stage, specifically
for this payoff.

kvstore was not benchmarked for range scans, and there's a real
structural reason a fair comparison would be harder to build: SSTables
are sorted *within* a run but scattered *across* runs, so a range scan
would need to merge across however many SSTables currently exist —
conceptually similar to the k-way merge used in compaction, but paid at
query time instead of amortized in the background. This is a genuine,
qualitative advantage for the B-tree's design that this project
didn't get to quantify, but can state with confidence based on how each
structure is laid out on disk.

## Summary

| | B-tree strength | LSM-tree strength |
|---|---|---|
| Reads (point) | — | Faster hits, dramatically faster misses (bloom filter) |
| Writes | — | Far fewer allocations at steady state (in-memory buffering) |
| Range scans | Leaf-chain sequential access, cost amortizes well | Not benchmarked; structurally more complex (multi-SSTable merge) |
| Missing acceleration | No bloom filter / early-exit for misses | — |

The two engines optimize for genuinely different workloads, and the
numbers bear that out: kvstore is the better choice when writes dominate
and point-lookup misses are common; btreedb's chained-leaf layout makes
it the natural choice when range scans are a first-class access pattern.
The clearest actionable next step surfaced by this comparison is adding
a bloom filter to btreedb's read path — the single highest-leverage
change suggested by the data.