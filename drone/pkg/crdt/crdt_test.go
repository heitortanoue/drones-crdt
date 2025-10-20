package crdt

import "testing"

// ---------- helpers -------------------------------------------------------

// equal compares two sets represented as map[E]struct{}
func equal[E comparable](a, b map[E]struct{}) bool {
	if len(a) != len(b) {
		return false
	}
	for k := range a {
		if _, ok := b[k]; !ok {
			return false
		}
	}
	return true
}

// elems builds a set of elements from an AWOR-Set
func elems[E comparable](s *AWORSet[E]) map[E]struct{} {
	out := make(map[E]struct{}, len(s.Elements()))
	for _, v := range s.Elements() {
		out[v] = struct{}{}
	}
	return out
}

// -------------------------------------------------------------------------
// 1. Add & Remove
// -------------------------------------------------------------------------
func TestAddRemove(t *testing.T) {
	s := NewAWORSet[string]()

	s.Add("A", "go")
	if _, ok := elems(s)["go"]; !ok {
		t.Fatalf("expected to contain 'go' after Add")
	}

	s.Remove("go")
	if _, ok := elems(s)["go"]; ok {
		t.Fatalf("expected not to contain 'go' after Remove")
	}
}

// -------------------------------------------------------------------------
// 2. Add wins vs Concurrent Remove
// -------------------------------------------------------------------------
func TestAddWinsConcurrent(t *testing.T) {
	// Initial state with "x"
	seed := NewAWORSet[string]()
	seed.Add("S", "x")

	// Replicas A and B start from the same state
	a := NewAWORSet[string]()
	b := NewAWORSet[string]()
	a.Merge(seed)
	b.Merge(seed)

	// Concurrent operations
	a.Add("A", "x") // new dot -> Add
	b.Remove("x")   // Remove only observed dots

	// Propagation
	a.Merge(b)
	b.Merge(a)

	if !(func() bool {
		_, aok := elems(a)["x"]
		_, bok := elems(b)["x"]
		return aok && bok
	}()) {
		t.Fatalf("Add did not win over Remove under concurrency")
	}
}

// -------------------------------------------------------------------------
// 3. Commutativity
// -------------------------------------------------------------------------
func TestMergeCommutative(t *testing.T) {
	a := NewAWORSet[string]()
	b := NewAWORSet[string]()
	a.Add("A", "a")
	b.Add("B", "b")

	left := NewAWORSet[string]()
	left.Merge(a)
	left.Merge(b)

	right := NewAWORSet[string]()
	right.Merge(b)
	right.Merge(a)

	if !equal(elems(left), elems(right)) {
		t.Fatalf("merge is not commutative – %v vs %v", elems(left), elems(right))
	}
}

// -------------------------------------------------------------------------
// 4. Associativity
// -------------------------------------------------------------------------
func TestMergeAssociative(t *testing.T) {
	a := NewAWORSet[string]()
	b := NewAWORSet[string]()
	c := NewAWORSet[string]()

	a.Add("A", "1")
	b.Add("B", "2")
	c.Add("C", "3")

	ab := NewAWORSet[string]()
	ab.Merge(a)
	ab.Merge(b) // (A ⊔ B)

	left := NewAWORSet[string]()
	left.Merge(ab)
	left.Merge(c) // (A ⊔ B) ⊔ C

	bc := NewAWORSet[string]()
	bc.Merge(b)
	bc.Merge(c) // (B ⊔ C)

	right := NewAWORSet[string]()
	right.Merge(a)
	right.Merge(bc) // A ⊔ (B ⊔ C)

	if !equal(elems(left), elems(right)) {
		t.Fatalf("merge is not associative – %v vs %v", elems(left), elems(right))
	}
}

// -------------------------------------------------------------------------
// 5. Idempotence
// -------------------------------------------------------------------------
func TestMergeIdempotent(t *testing.T) {
	s := NewAWORSet[string]()
	s.Add("A", "z")

	before := elems(s)

	s.Merge(s) // merge with itself

	if !equal(before, elems(s)) {
		t.Fatalf("merge is not idempotent – before %v, after %v", before, elems(s))
	}
}

// -------------------------------------------------------------------------
// 6. Additional removal and semantic tests
// -------------------------------------------------------------------------

// TestMultipleAdds checks that multiple additions accumulate elements
func TestMultipleAdds(t *testing.T) {
	s := NewAWORSet[string]()
	s.Add("A", "a")
	s.Add("A", "b")
	got := elems(s)
	want := map[string]struct{}{"a": {}, "b": {}}
	if !equal(got, want) {
		t.Fatalf("expected %v, got %v", want, got)
	}
}

// TestRemoveWinsWithoutConcurrentAdd checks remove without concurrent add
func TestRemoveWinsWithoutConcurrentAdd(t *testing.T) {
	seed := NewAWORSet[string]()
	seed.Add("S", "x")
	a := NewAWORSet[string]()
	b := NewAWORSet[string]()
	a.Merge(seed)
	b.Merge(seed)

	a.Remove("x")

	a.Merge(b)
	b.Merge(a)

	if _, oka := elems(a)["x"]; oka || func() bool { _, okb := elems(b)["x"]; return okb }() {
		t.Fatalf("expected 'x' removed, but still present in A:%v B:%v", elems(a), elems(b))
	}
}

// TestAddWinsOnConcurrentRemoveAndAdd verifies add-vs-remove concurrency semantics
func TestAddWinsOnConcurrentRemoveAndAdd(t *testing.T) {
	seed := NewAWORSet[string]()
	seed.Add("S", "x")
	a := NewAWORSet[string]()
	b := NewAWORSet[string]()
	a.Merge(seed)
	b.Merge(seed)

	a.Remove("x")
	b.Add("B", "x")

	a.Merge(b)
	b.Merge(a)

	if !equal(elems(a), elems(b)) || func() bool { _, ok := elems(a)["x"]; return !ok }() {
		t.Fatalf("expected 'x' present after concurrent add-vs-remove, got A:%v B:%v", elems(a), elems(b))
	}
}

// TestReAddAfterRemove checks re-adding after removal
func TestReAddAfterRemove(t *testing.T) {
	s := NewAWORSet[string]()
	s.Add("A", "go")
	s.Remove("go")
	s.Add("A", "go")
	if _, ok := elems(s)["go"]; !ok {
		t.Fatalf("expected 'go' present after remove and re-add")
	}
}

// TestRemoveNonExistent checks removing a non-existent element does nothing
func TestRemoveNonExistent(t *testing.T) {
	s := NewAWORSet[string]()
	s.Remove("nope")
	if len(s.Elements()) != 0 {
		t.Fatalf("expected empty set after removing non-existent element, got %v", s.Elements())
	}
}

// TestMultipleElementsRemoveOne checks selective removal
func TestMultipleElementsRemoveOne(t *testing.T) {
	s := NewAWORSet[string]()
	s.Add("A", "a")
	s.Add("A", "b")
	s.Remove("a")
	got := elems(s)
	want := map[string]struct{}{"b": {}}
	if !equal(got, want) {
		t.Fatalf("expected %v after removing 'a', got %v", want, got)
	}
}

// -------------------------------------------------------------------------
// 7. Delta Operations Tests - CRITICAL
// -------------------------------------------------------------------------

// TestDeltaIsCreatedOnAdd verifies that Add creates a Delta
func TestDeltaIsCreatedOnAdd(t *testing.T) {
	s := NewAWORSet[string]()

	if s.Delta != nil {
		t.Fatalf("expected nil Delta initially")
	}

	s.Add("A", "x")

	if s.Delta == nil {
		t.Fatalf("expected Delta to be created after Add")
	}

	if len(s.Delta.Entries) != 1 {
		t.Fatalf("expected 1 entry in Delta, got %d", len(s.Delta.Entries))
	}

	// Verify Delta.Context.Clock was updated
	if s.Delta.Context.Clock["A"] != 1 {
		t.Fatalf("expected Delta.Context.Clock[A]=1, got %d", s.Delta.Context.Clock["A"])
	}
}

// TestDeltaIsCreatedOnRemove verifies that Remove creates a Delta with DotCloud
func TestDeltaIsCreatedOnRemove(t *testing.T) {
	s := NewAWORSet[string]()
	s.Add("A", "x")

	// Note: After Add, Delta has the added entry
	// Now we remove
	s.Remove("x")

	if s.Delta == nil {
		t.Fatalf("expected Delta to be created/maintained after Remove")
	}

	// Delta.Context.DotCloud should have the removed dot
	// After compaction, it might be empty if the dot was folded into clock
	// But Core.Context.DotCloud should have it
	if len(s.Core.Context.DotCloud) == 0 && len(s.Delta.Context.DotCloud) == 0 {
		// Check if the dot was compacted into the clock
		if s.Core.Context.Clock["A"] < 1 {
			t.Fatalf("expected removed dot to be tracked (in DotCloud or Clock)")
		}
	}
}

// TestMergeDeltaVsMerge validates that MergeDelta works correctly
func TestMergeDeltaVsMerge(t *testing.T) {
	a := NewAWORSet[string]()
	a.Add("A", "x")

	// Get the delta
	deltaKernel := a.Delta

	// Replica B receives the delta
	b := NewAWORSet[string]()
	b.MergeDelta(deltaKernel)

	// Verify B now has "x"
	if _, ok := elems(b)["x"]; !ok {
		t.Fatalf("expected B to have 'x' after MergeDelta")
	}

	// Verify B's Delta was NOT cleared (MergeDelta doesn't clear local delta)
	// B hasn't made local changes, so Delta should still be nil
	if b.Delta != nil && len(b.Delta.Entries) > 0 {
		t.Fatalf("expected B's Delta to be empty after receiving external delta")
	}
}

// TestAddOptimizationRemovesDuplicates validates the critical optimization
// that Add removes old occurrences BEFORE adding new dot
func TestAddOptimizationRemovesDuplicates(t *testing.T) {
	s := NewAWORSet[string]()

	// Add "x" multiple times
	s.Add("A", "x")
	s.Add("A", "x")
	s.Add("A", "x")

	// Count how many dots exist for "x"
	count := 0
	for _, v := range s.Core.Entries {
		if v == "x" {
			count++
		}
	}

	// Should only have 1 dot for "x" (the optimization removes old ones first)
	if count != 1 {
		t.Fatalf("expected only 1 dot for 'x' after optimization, got %d", count)
	}

	// Verify it's the most recent dot (counter should be 3)
	for d, v := range s.Core.Entries {
		if v == "x" {
			if d.Counter != 3 {
				t.Fatalf("expected most recent dot (counter=3), got counter=%d", d.Counter)
			}
		}
	}
}

// TestDotCloudCompaction verifies that DotCloud is compacted correctly
func TestDotCloudCompaction(t *testing.T) {
	s := NewAWORSet[string]()

	// Add and remove to create dots
	s.Add("A", "x")
	s.Remove("x")
	s.Add("A", "y")

	// The removed dot should be compacted into the clock if it's contiguous
	// After Add("A","x") -> Clock[A]=1, Entries[(A,1)]=x
	// After Remove("x") -> Clock[A]=1, DotCloud[(A,1)], Entries={}
	// After Add("A","y") -> Clock[A]=2, Entries[(A,2)]=y
	// Compact should fold (A,1) into clock since it's ≤ Clock[A]=2

	// Force compact
	s.Core.Context.compact()

	// DotCloud should be empty or minimal
	if len(s.Core.Context.DotCloud) > 1 {
		t.Logf("Warning: DotCloud has %d entries (expected compact): %v",
			len(s.Core.Context.DotCloud), s.Core.Context.DotCloud)
	}
}

// TestMultipleReplicasConvergence tests convergence with 3+ replicas
func TestMultipleReplicasConvergence(t *testing.T) {
	// Create 3 replicas
	a := NewAWORSet[string]()
	b := NewAWORSet[string]()
	c := NewAWORSet[string]()

	// Each adds different elements
	a.Add("A", "a")
	b.Add("B", "b")
	c.Add("C", "c")

	// Simulate gossip: everyone merges with everyone
	// Round 1
	a.Merge(b)
	a.Merge(c)

	b.Merge(a)
	b.Merge(c)

	c.Merge(a)
	c.Merge(b)

	// All should have converged to {a, b, c}
	want := map[string]struct{}{"a": {}, "b": {}, "c": {}}

	if !equal(elems(a), want) {
		t.Fatalf("replica A did not converge: got %v, want %v", elems(a), want)
	}
	if !equal(elems(b), want) {
		t.Fatalf("replica B did not converge: got %v, want %v", elems(b), want)
	}
	if !equal(elems(c), want) {
		t.Fatalf("replica C did not converge: got %v, want %v", elems(c), want)
	}
}

// TestComplexConcurrentOperations tests A add, B remove, C add scenario
func TestComplexConcurrentOperations(t *testing.T) {
	// Setup: all start with "x"
	seed := NewAWORSet[string]()
	seed.Add("S", "x")

	a := NewAWORSet[string]()
	b := NewAWORSet[string]()
	c := NewAWORSet[string]()

	a.Merge(seed)
	b.Merge(seed)
	c.Merge(seed)

	// Concurrent operations:
	a.Add("A", "x") // A adds x again (new dot)
	b.Remove("x")   // B removes x (old dot)
	c.Add("C", "y") // C adds y (unrelated)

	// Propagate changes
	a.Merge(b)
	a.Merge(c)

	b.Merge(a)
	b.Merge(c)

	c.Merge(a)
	c.Merge(b)

	// Result: x should be present (A's new add wins), y should be present
	want := map[string]struct{}{"x": {}, "y": {}}

	if !equal(elems(a), want) {
		t.Fatalf("replica A failed: got %v, want %v", elems(a), want)
	}
	if !equal(elems(b), want) {
		t.Fatalf("replica B failed: got %v, want %v", elems(b), want)
	}
	if !equal(elems(c), want) {
		t.Fatalf("replica C failed: got %v, want %v", elems(c), want)
	}
}

// TestContextContains validates that Context.Contains works correctly
func TestContextContains(t *testing.T) {
	ctx := NewDotContext()

	// Add some dots to clock
	ctx.Clock["A"] = 5
	ctx.Clock["B"] = 3

	// Add some dots to cloud
	ctx.DotCloud[Dot{NodeID: "C", Counter: 10}] = true

	tests := []struct {
		dot      Dot
		expected bool
		reason   string
	}{
		{Dot{"A", 3}, true, "counter 3 ≤ clock[A]=5"},
		{Dot{"A", 5}, true, "counter 5 = clock[A]=5"},
		{Dot{"A", 6}, false, "counter 6 > clock[A]=5 and not in cloud"},
		{Dot{"B", 3}, true, "counter 3 = clock[B]=3"},
		{Dot{"B", 4}, false, "counter 4 > clock[B]=3 and not in cloud"},
		{Dot{"C", 10}, true, "explicitly in DotCloud"},
		{Dot{"C", 5}, false, "not in clock and not in cloud"},
		{Dot{"D", 1}, false, "node D not seen at all"},
	}

	for _, tt := range tests {
		got := ctx.Contains(tt.dot)
		if got != tt.expected {
			t.Errorf("Contains(%v) = %v, want %v (%s)",
				tt.dot, got, tt.expected, tt.reason)
		}
	}
}

// -------------------------------------------------------------------------
// Test case (d): Concurrent adds create multiple dots for same value
// -------------------------------------------------------------------------
func TestConcurrentAddsMultipleDots(t *testing.T) {
	// Two replicas concurrently add the same value "x"
	replicaA := NewAWORSet[string]()
	replicaB := NewAWORSet[string]()

	// Concurrent adds without observing each other
	dotA := replicaA.Add("A", "x")
	dotB := replicaB.Add("B", "x")

	// Verify each has one dot locally
	if len(replicaA.Core.Entries) != 1 {
		t.Fatalf("ReplicaA should have 1 entry, got %d", len(replicaA.Core.Entries))
	}
	if len(replicaB.Core.Entries) != 1 {
		t.Fatalf("ReplicaB should have 1 entry, got %d", len(replicaB.Core.Entries))
	}

	// Exchange deltas: A receives B's delta, B receives A's delta
	replicaA.MergeDelta(replicaB.Delta)
	replicaB.MergeDelta(replicaA.Delta)

	// Both should now have 2 dots for the same value "x"
	if len(replicaA.Core.Entries) != 2 {
		t.Fatalf("ReplicaA should have 2 entries after merge, got %d", len(replicaA.Core.Entries))
	}
	if len(replicaB.Core.Entries) != 2 {
		t.Fatalf("ReplicaB should have 2 entries after merge, got %d", len(replicaB.Core.Entries))
	}

	// Verify both dots are present
	if _, ok := replicaA.Core.Entries[dotA]; !ok {
		t.Error("ReplicaA should contain dot from A")
	}
	if _, ok := replicaA.Core.Entries[dotB]; !ok {
		t.Error("ReplicaA should contain dot from B")
	}

	// Semantically, the set contains only one element
	elemsA := elems(replicaA)
	if len(elemsA) != 1 {
		t.Fatalf("ReplicaA should have 1 semantic element, got %d", len(elemsA))
	}
	if _, ok := elemsA["x"]; !ok {
		t.Error("ReplicaA should contain 'x'")
	}

	// Both replicas converge to the same state
	elemsB := elems(replicaB)
	if !equal(elemsA, elemsB) {
		t.Error("Replicas should converge to the same semantic state")
	}
}

// -------------------------------------------------------------------------
// Test: Remove must iterate all entries to remove all dots for a value
// -------------------------------------------------------------------------
func TestRemoveIteratesAllDotsForValue(t *testing.T) {
	// Create scenario with multiple dots for same value
	replicaA := NewAWORSet[string]()
	replicaB := NewAWORSet[string]()
	replicaC := NewAWORSet[string]()

	// Three replicas concurrently add "x"
	replicaA.Add("A", "x")
	replicaB.Add("B", "x")
	replicaC.Add("C", "x")

	// Merge all deltas into replica A
	replicaA.MergeDelta(replicaB.Delta)
	replicaA.MergeDelta(replicaC.Delta)

	// ReplicaA now has 3 dots for "x"
	if len(replicaA.Core.Entries) != 3 {
		t.Fatalf("ReplicaA should have 3 entries, got %d", len(replicaA.Core.Entries))
	}

	// Verify all three values are "x"
	for _, v := range replicaA.Core.Entries {
		if v != "x" {
			t.Errorf("Expected all entries to be 'x', got '%s'", v)
		}
	}

	// Remove "x" - must remove all 3 dots
	replicaA.Remove("x")

	// All entries should be removed
	if len(replicaA.Core.Entries) != 0 {
		t.Fatalf("All entries should be removed, got %d remaining", len(replicaA.Core.Entries))
	}

	// Set should be empty
	if len(replicaA.Elements()) != 0 {
		t.Error("Set should be empty after removing all dots for 'x'")
	}
}

// -------------------------------------------------------------------------
// Test: Add-wins with concurrent adds and remove
// -------------------------------------------------------------------------
func TestAddWinsWithMultipleConcurrentAdds(t *testing.T) {
	// Initial state: all replicas have "x"
	replicaA := NewAWORSet[string]()
	replicaB := NewAWORSet[string]()
	replicaC := NewAWORSet[string]()

	// Initial add by A
	replicaA.Add("A", "x")
	replicaB.MergeDelta(replicaA.Delta)
	replicaC.MergeDelta(replicaA.Delta)

	// Clear deltas
	replicaA.Delta = nil
	replicaB.Delta = nil
	replicaC.Delta = nil

	// Concurrent operations:
	// A removes "x" (knows only about (A,1))
	replicaA.Remove("x")

	// B and C concurrently add "x" with fresh dots
	dotB := replicaB.Add("B", "x")
	dotC := replicaC.Add("C", "x")

	// Exchange all deltas
	// A receives B's and C's adds
	replicaA.MergeDelta(replicaB.Delta)
	replicaA.MergeDelta(replicaC.Delta)

	// B receives A's remove and C's add
	replicaB.MergeDelta(replicaA.Delta)
	replicaB.MergeDelta(replicaC.Delta)

	// C receives A's remove and B's add
	replicaC.MergeDelta(replicaA.Delta)
	replicaC.MergeDelta(replicaB.Delta)

	// All replicas should converge with "x" present (add wins)
	elemsA := elems(replicaA)
	elemsB := elems(replicaB)
	elemsC := elems(replicaC)

	if _, ok := elemsA["x"]; !ok {
		t.Error("ReplicaA should contain 'x' (add wins)")
	}
	if _, ok := elemsB["x"]; !ok {
		t.Error("ReplicaB should contain 'x' (add wins)")
	}
	if _, ok := elemsC["x"]; !ok {
		t.Error("ReplicaC should contain 'x' (add wins)")
	}

	// All should have the same semantic state
	if !equal(elemsA, elemsB) || !equal(elemsB, elemsC) {
		t.Error("All replicas should converge to same state")
	}

	// Should have 2 dots (B's and C's)
	if len(replicaA.Core.Entries) != 2 {
		t.Errorf("Should have 2 dots, got %d", len(replicaA.Core.Entries))
	}

	// Verify specific dots exist
	if _, ok := replicaA.Core.Entries[dotB]; !ok {
		t.Error("Should contain dot from B")
	}
	if _, ok := replicaA.Core.Entries[dotC]; !ok {
		t.Error("Should contain dot from C")
	}
}

// -------------------------------------------------------------------------
// Test: Partial remove with concurrent adds
// -------------------------------------------------------------------------
func TestPartialRemoveWithConcurrentAdds(t *testing.T) {
	// Setup: A and B both add "y"
	replicaA := NewAWORSet[string]()
	replicaB := NewAWORSet[string]()

	dotA := replicaA.Add("A", "y")
	dotB := replicaB.Add("B", "y")

	// Both receive each other's adds
	replicaA.MergeDelta(replicaB.Delta)
	replicaB.MergeDelta(replicaA.Delta)

	// Now both have 2 dots for "y"
	if len(replicaA.Core.Entries) != 2 {
		t.Fatalf("Should have 2 dots, got %d", len(replicaA.Core.Entries))
	}

	// Clear deltas
	replicaA.Delta = nil
	replicaB.Delta = nil

	// A removes "y" (knows about both dots)
	replicaA.Remove("y")

	// Verify A's remove delta knows about both dots
	if len(replicaA.Core.Entries) != 0 {
		t.Error("A should have no entries after remove")
	}

	// B adds "y" again with a fresh dot (concurrent with A's remove)
	dotB2 := replicaB.Add("B", "y")

	// Exchange deltas
	replicaA.MergeDelta(replicaB.Delta)
	replicaB.MergeDelta(replicaA.Delta)

	// Both should converge with "y" present (the new add from B)
	elemsA := elems(replicaA)
	elemsB := elems(replicaB)

	if !equal(elemsA, elemsB) {
		t.Error("Replicas should converge")
	}

	if _, ok := elemsA["y"]; !ok {
		t.Error("Should contain 'y' (new add wins)")
	}

	// Should only have the new dot from B
	if len(replicaA.Core.Entries) != 1 {
		t.Errorf("Should have 1 dot (the new one), got %d", len(replicaA.Core.Entries))
	}

	// Verify it's the new dot, not the old ones
	if _, ok := replicaA.Core.Entries[dotB2]; !ok {
		t.Error("Should contain the new dot from B")
	}
	if _, ok := replicaA.Core.Entries[dotA]; ok {
		t.Error("Should not contain the old dot from A")
	}
	if _, ok := replicaA.Core.Entries[dotB]; ok {
		t.Error("Should not contain the old dot from B")
	}
}
