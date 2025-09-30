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