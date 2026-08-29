package btreestore

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"testing"
)

/*
Benchmark Notes: btreedb

Methodology. All comparisons use fixed iteration counts (-benchtime=Nx), never auto-scaled b.N, when comparing two variants against each other — auto-scaling lets each variant run a different number of iterations, and since Put cost scales with tree state, this alone can flip which variant looks faster (caught this directly: an early "checkpointing makes Put slower" result turned out to be exactly this artifact). Get/Get-miss/RangeGet share a single pre-populated store via sync.Once so comparisons run against an identical tree, not separately-built ones.

Finding 1 — Get-hit and Get-miss cost the same. ~6.4μs, ~241 allocs/op, both. No early-exit optimization makes a miss cheaper — both pay the same full root-to-leaf descent. Worth contrasting directly with the LSM-tree, where the bloom filter exists specifically to make misses cheaper than hits.

Finding 2 — checkpointing frequency is not the dominant cost of Put. Across fixed-N comparisons (100x/1000x/10000x) between a threshold-30000 store and an effectively-checkpoint-disabled one, the two are statistically indistinguishable — whichever "wins" flips depending on N, indicating noise. The pager-sync tradeoff introduced in step 7 exists for correctness/performance reasons that matter at a different layer (crash durability window), not because it dominates measured Put latency here.

Finding 3 — allocation cost is driven by entries-per-leaf, not tree depth. A short-key tree (10,000 inserts, depth 1, 53 leaves) shows more allocs/op (301) than a long-key tree at the same insert count (depth 3, 1,820 leaves, 97 allocs/op) — despite the deep tree touching far more pages per operation. This points at the leaf's in-memory slice splice (insertLeaf's shift-and-reallocate) as the real allocation cost center, not descent depth. Concrete, measured evidence for where the deferred full-rebuild-vs-incremental-edit optimization (KNOWN_LIMITATIONS.md) would actually pay off.

Finding 4 — range scans amortize well; writes are the expensive operation. Per-entry cost drops ~5.6x going from a 10-entry scan (~747 ns/entry) to a 5,000-entry scan (~133 ns/entry) — the fixed descent cost is paid once, then each additional entry is nearly free (sequential leaf-chain reads via nextPageID). By contrast, a single Put/Delete (~2.5-2.8ms) costs roughly 4x what it costs to scan 5,000 keys — the gap is the full page encode/write cost that reads never pay. This is the clearest structural argument for the B-tree's range-scan advantage over the LSM-tree, and the clearest argument for why writes are the side worth optimizing first.

Raw numbers (darwin/amd64, VirtualApple @ 2.50GHz):

Benchmark	ns/op	B/op	allocs/op
Put	2,784,260	23,359	256
Put (checkpointing disabled)	2,801,616	23,398	259
Get (hit)	6,383	20,931	241
Get (miss)	6,345	20,937	242
RangeGet (10 keys)	7,468	23,396	271
RangeGet (5,000 keys)	664,315	1,279,684	10,487
Delete	2,546,730	19,599	149
Put — shallow tree (depth 1)	2,901,056	37,780	301
Put — deep tree (depth 3)	3,118,007	39,448	97
*/

/*
BenchmarkStore_Put-12                          	     394	   2784260 ns/op	   23359 B/op	     256 allocs/op
BenchmarkStore_Put_With_No_CheckPointing-12    	     448	   2801616 ns/op	   23398 B/op	     259 allocs/op
BenchmarkStore_Get-12                          	  189572	      6383 ns/op	   20931 B/op	     241 allocs/op
BenchmarkStore_Get_Miss-12                     	  186880	      6345 ns/op	   20937 B/op	     242 allocs/op
BenchmarkStore_RangeGet_Small_Range-12         	  161314	      7468 ns/op	   23396 B/op	     271 allocs/op
BenchmarkStore_RangeGet_Large_Range-12         	    1904	    664315 ns/op	 1279684 B/op	   10487 allocs/op
BenchmarkStore_Delete-12                       	     783	   2546730 ns/op	   19599 B/op	     149 allocs/op

*/

// Benchmark methodology and findings, btreedb vs. planned LSM-tree comparison.
//
// SETUP
//   - Put benchmarks use their own freshly-created store per run.
//   - Get/Get_Miss share a single pre-populated store (30,001 keys,
//     via sync.Once) so both measure against an identical tree shape -
//     comparing them against separately-built stores produced a
//     misleading ~20% gap that vanished once the store was shared and
//     key formats were made structurally identical (see history below).
//
// FINDINGS
//
//  1. Get-hit and Get-miss cost the same (~6.3-6.5μs, ~241 allocs/op).
//     Expected: there's no early-exit optimization (e.g. a bloom
//     filter) that makes a miss cheaper - both pay the full
//     root-to-leaf descent regardless of outcome. Worth highlighting
//     against the LSM-tree, where the bloom filter exists specifically
//     to make misses cheaper than hits.
//
//  2. Checkpointing frequency is NOT the dominant cost of Put.
//     Comparing checkpointed (threshold=30000) vs. effectively-disabled
//     checkpointing at fixed iteration counts (-benchtime=100x/1000x/10000x)
//     shows no consistent difference - whichever variant looks faster
//     flips depending on N, indicating noise, not a real effect.
//     IMPORTANT: only trust fixed -benchtime=Nx comparisons here.
//     Auto-scaled b.N lets the two variants run different iteration
//     counts, which - since Put cost scales with tree depth (see #3) -
//     makes one look faster or slower purely as an artifact of how
//     deep each tree happened to grow. This produced a misleading
//     "checkpointing makes Put slower" result before being caught.
//
//  3. Put cost and allocs/op scale with tree depth, not checkpointing.
//     Fixed-N runs show allocs/op climbing from ~107 (N=100) to ~335
//     (N=10000) regardless of checkpoint setting. This points at the
//     full-page-rebuild-per-insert design (every encode() allocates a
//     fresh 4KB buffer, every insertLeaf splice reallocates the kv
//     slice) as the real cost driver as the tree grows deeper - a
//     concrete, measured case for the deferred "incremental in-place
//     edit vs. full rebuild" optimization noted in KNOWN_LIMITATIONS.md.
//

var (
	sharedStore     *Store
	sharedStoreOnce sync.Once
	deleteCursor    int = 0
)

const keyNumPrefix = 10000000
const threshold = 30000

func getSharedStore(b *testing.B) *Store {
	sharedStoreOnce.Do(func() {
		store := createStore(b, threshold)

		for i := keyNumPrefix; i < keyNumPrefix+threshold+1; i++ {
			err := store.Put(fmt.Sprintf("key-%d", i), fmt.Sprintf("value-%d", i))
			if err != nil {
				b.Fatalf("store.Put failed %v", err)
			}
		}
		sharedStore = store
	})
	return sharedStore
}

func createStore(b *testing.B, thresholdOverride int) *Store {
	dir, err := os.MkdirTemp("", "bench")
	if err != nil {
		b.Fatal(err)
	}

	dbPath := filepath.Join(dir, "test.db")
	walPath := filepath.Join(dir, "test.wal")

	options := StoreOptions{
		DBPath:                 dbPath,
		WalPath:                walPath,
		CheckpointingThreshold: thresholdOverride,
	}
	store, err := Open(options)
	if err != nil {
		b.Fatal(err)
	}
	return store
}

/*
✗ go test -bench=. -benchmem -run=^$ -benchtime=100x
BenchmarkStore_Put-12                          	     100	   2752150 ns/op	   12526 B/op	     107 allocs/op
✗ go test -bench=. -benchmem -run=^$ -benchtime=1000x
BenchmarkStore_Put-12                          	    1000	   2950965 ns/op	   26162 B/op	     286 allocs/op
✗ go test -bench=. -benchmem -run=^$ -benchtime=10000x
BenchmarkStore_Put-12                          	   10000	   2825069 ns/op	   29162 B/op	     335 allocs/op
*/

func BenchmarkStore_Put(b *testing.B) {
	putBench(b, 100)
}

/*
✗ go test -bench=. -benchmem -run=^$ -benchtime=100x
BenchmarkStore_Put_With_No_CheckPointing-12    	     100	   2920742 ns/op	   12526 B/op	     108 allocs/op
✗ go test -bench=. -benchmem -run=^$ -benchtime=1000x
BenchmarkStore_Put_With_No_CheckPointing-12    	    1000	   2794481 ns/op	   26156 B/op	     286 allocs/op
✗ go test -bench=. -benchmem -run=^$ -benchtime=10000x
BenchmarkStore_Put_With_No_CheckPointing-12    	   10000	   2391759 ns/op	   29162 B/op	     335 allocs/op
*/
func BenchmarkStore_Put_With_No_CheckPointing(b *testing.B) {
	putBench(b, 10000000)

}

func putBench(b *testing.B, threshold int) {
	store := createStore(b, threshold)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		err := store.Put(fmt.Sprintf("key-%d", i), fmt.Sprintf("value-%d", i))
		if err != nil {
			b.Fatalf("store.Put failed %v", err)
		}
	}
}

/*
BenchmarkStore_Get-12         	  152570	      7698 ns/op	   22937 B/op	     313 allocs/op
*/
func BenchmarkStore_Get(b *testing.B) {

	store := getSharedStore(b)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("key-%d", keyNumPrefix+i%threshold)
		_, err := store.Get(key)
		if err != nil && !errors.Is(err, NotFound) {
			b.Fatalf("store.Get failed %v", err)
		}
	}
}

/*
BenchmarkStore_Get_Miss-12    	  126339	      9362 ns/op	   26359 B/op	     449 allocs/op
*/
func BenchmarkStore_Get_Miss(b *testing.B) {
	store := getSharedStore(b)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("key-%d0", keyNumPrefix+i%threshold)
		_, err := store.Get(key)
		if err != nil && !errors.Is(err, NotFound) {
			b.Fatalf("store.Get failed %v", err)
		}
	}
}

func BenchmarkStore_RangeGet_Small_Range(b *testing.B) {
	store := getSharedStore(b)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		offset := i % (threshold - 10) // keep start+10 within the populated range
		start := fmt.Sprintf("key-%d", keyNumPrefix+offset)
		end := fmt.Sprintf("key-%d", keyNumPrefix+offset+10)
		_, err := store.RangeGet(start, end)
		if err != nil {
			b.Fatalf("store.Get failed %v", err)
		}
	}
}

/*
Small range: 10 entries, 7,468 ns/op → ~747 ns/entry.
Large range: 5,000 entries, 664,315 ns/op → ~133 ns/entry.
*/
func BenchmarkStore_RangeGet_Large_Range(b *testing.B) {
	store := getSharedStore(b)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		offset := i % (threshold - 5000) // keep start+10 within the populated range
		start := fmt.Sprintf("key-%d", keyNumPrefix+offset)
		end := fmt.Sprintf("key-%d", keyNumPrefix+offset+5000)
		_, err := store.RangeGet(start, end)
		if err != nil {
			b.Fatalf("store.Get failed %v", err)
		}
	}
}

func treeDeptBenchmarking(b *testing.B, baseKey string, baseValue string, caller string) {
	store := createStore(b, 100)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := store.Put(baseKey+strconv.Itoa(i), baseValue)
		if err != nil {
			b.Fatalf("store.Put failed %v", err)
		}
	}
	b.StopTimer()

	td := &TestData{}
	if err := dumpTree(store, store.superBlock.rootPageID, 0, td); err != nil {
		b.Fatal(err)
	}
	if len(td.leafDepths) > 0 {
		b.Logf("tree depth of %s : %d, leaf count: %d", caller, td.leafDepths[0], len(td.leafDepths))
	}
}

/*
BenchmarkStore_ShallowBtree-12             	   10000	   2683583 ns/op	   37780 B/op	     301 allocs/op
--- BENCH: BenchmarkStore_ShallowBtree-12
store_benchmark_test.go:236: tree depth of ShallowBtree : 0, leaf count: 1
store_benchmark_test.go:236: tree depth of ShallowBtree : 1, leaf count: 53
BenchmarkStore_IntermediateDeepBtree-12    	   10000	   3072438 ns/op	   31571 B/op	     227 allocs/op
--- BENCH: BenchmarkStore_IntermediateDeepBtree-12
store_benchmark_test.go:236: tree depth of DeepBtree : 0, leaf count: 1
store_benchmark_test.go:236: tree depth of DeepBtree : 2, leaf count: 301
BenchmarkStore_DeepBtree-12                	   10000	   3132513 ns/op	   39448 B/op	      97 allocs/op
--- BENCH: BenchmarkStore_DeepBtree-12
store_benchmark_test.go:236: tree depth of DeepBtree : 0, leaf count: 1
store_benchmark_test.go:236: tree depth of DeepBtree : 3, leaf count: 1820

*/

// a direct shallow-tree-vs-deep-tree
// Put comparison isolating depth as the only variable.
func BenchmarkStore_ShallowBtree(b *testing.B) {
	baseKey := "k"
	baseValue := "i"
	treeDeptBenchmarking(b, baseKey, baseValue, "ShallowBtree")
}

func BenchmarkStore_IntermediateDeepBtree(b *testing.B) {
	baseKey := "keydkjngdfshfkhkhfjhkdfhksdhvkdh"
	baseValue := "i am a very very laaaa"

	treeDeptBenchmarking(b, baseKey, baseValue, "DeepBtree")
}
func BenchmarkStore_DeepBtree(b *testing.B) {
	baseKey := "keydkjngdfshfkhkhfjhkdfhksdhvkdhkdfkdhfsdhfkdhfkdsjhfksdfhksfsasjashkdfhxkcdkjngdfshfkhkhfjhkdfhksdhvkdhkdfkdhfsdhfkdhfkdsjhfksdfhksfsasjashkdfhxkc"
	baseValue := "i am a very very laaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaage value, i am a very very laaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaage value, i am a very very laaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaage value"

	treeDeptBenchmarking(b, baseKey, baseValue, "DeepBtree")
}
func BenchmarkStore_Delete(b *testing.B) {
	store := getSharedStore(b)

	b.ResetTimer()
	for ; deleteCursor < b.N; deleteCursor++ {
		if deleteCursor >= threshold {
			b.Fatalf("delete benchmark exhausted the pre-populated key pool (threshold=%d)", threshold)
		}
		key := fmt.Sprintf("key-%d", keyNumPrefix+deleteCursor)
		err := store.Delete(key)
		if err != nil {
			b.Fatalf("store.Delete failed %v", err)
		}
	}
}
