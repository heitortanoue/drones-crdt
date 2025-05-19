package crdt

import "testing"

// ---------- helpers -------------------------------------------------------

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

// copia lógica – produz um conjunto de elementos a partir de um AWOR-Set
func elems[E comparable](s *AWORSet[E]) map[E]struct{} { return s.Elements() }

// -------------------------------------------------------------------------
// 1. Add & Remove
// -------------------------------------------------------------------------
func TestAddRemove(t *testing.T) {
	s := New[string]("A")

	s.Add("go")
	if !s.Contains("go") {
		t.Fatalf("esperava conter 'go' após Add")
	}

	s.Remove("go")
	if s.Contains("go") {
		t.Fatalf("esperava não conter 'go' após Remove")
	}
}

// -------------------------------------------------------------------------
// 2. Add wins x Remove concorrente
// -------------------------------------------------------------------------
func TestAddWinsConcurrent(t *testing.T) {
	// Estado inicial com "x"
	seed := New[string]("S")
	seed.Add("x")

	// Réplicas A e B partem do mesmo estado
	a := New[string]("A")
	b := New[string]("B")
	a.Merge(seed)
	b.Merge(seed)

	// Operações concorrentes…
	a.Add("x")    // novo dot ⟶ Add
	b.Remove("x") // Remove só dots já observados

	// …e propagação
	a.Merge(b)
	b.Merge(a)

	if !(a.Contains("x") && b.Contains("x")) {
		t.Fatalf("Add não venceu Remove na presença de concorrência")
	}
}

// -------------------------------------------------------------------------
// 3. Conmutatividade
// -------------------------------------------------------------------------
func TestMergeCommutative(t *testing.T) {
	a := New[string]("A")
	b := New[string]("B")
	a.Add("a")
	b.Add("b")

	left := New[string]("L")
	left.Merge(a)
	left.Merge(b)

	right := New[string]("R")
	right.Merge(b)
	right.Merge(a)

	if !equal(elems(left), elems(right)) {
		t.Fatalf("merge não é comutativo – %v vs %v", elems(left), elems(right))
	}
}

// -------------------------------------------------------------------------
// 4. Associatividade
// -------------------------------------------------------------------------
func TestMergeAssociative(t *testing.T) {
	a := New[string]("A")
	b := New[string]("B")
	c := New[string]("C")

	a.Add("1")
	b.Add("2")
	c.Add("3")

	ab := New[string]("ab")
	ab.Merge(a)
	ab.Merge(b) // (A ⊔ B)

	left := New[string]("left")
	left.Merge(ab)
	left.Merge(c) // (A ⊔ B) ⊔ C

	bc := New[string]("bc")
	bc.Merge(b)
	bc.Merge(c) // (B ⊔ C)

	right := New[string]("right")
	right.Merge(a)
	right.Merge(bc) // A ⊔ (B ⊔ C)

	if !equal(elems(left), elems(right)) {
		t.Fatalf("merge não é associativo – %v vs %v", elems(left), elems(right))
	}
}

// -------------------------------------------------------------------------
// 5. Idempotência
// -------------------------------------------------------------------------
func TestMergeIdempotent(t *testing.T) {
	s := New[string]("A")
	s.Add("z")

	before := elems(s)

	s.Merge(s) // merge consigo mesmo

	if !equal(before, elems(s)) {
		t.Fatalf("merge não é idempotente – antes %v, depois %v", before, elems(s))
	}
}

// -------------------------------------------------------------------------
// 6. Equivalência Estado × Delta
// -------------------------------------------------------------------------
func TestDeltaMergeEquivalence(t *testing.T) {
	// Nó A faz duas operações e envia só os deltas
	a := New[string]("A")
	d1 := a.Add("alpha") // Δ₁
	d2 := a.Add("beta")  // Δ₂

	// Nó B recebe apenas os deltas
	b := New[string]("B")
	b.Merge(d1)
	b.Merge(d2)

	// Nó C recebe o estado completo (por ex. sincronização eventual)
	c := New[string]("C")
	c.Merge(a)

	if !equal(elems(b), elems(c)) {
		t.Fatalf("delta merge ≠ full-state merge – B:%v C:%v", elems(b), elems(c))
	}
}