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
		t.Fatalf("esperava conter 'go' após Add")
	}

	s.Remove("go")
	if _, ok := elems(s)["go"]; ok {
		t.Fatalf("esperava não conter 'go' após Remove")
	}
}

// -------------------------------------------------------------------------
// 2. Add wins x Remove concorrente
// -------------------------------------------------------------------------
func TestAddWinsConcurrent(t *testing.T) {
	// Estado inicial com "x"
	seed := NewAWORSet[string]()
	seed.Add("S", "x")

	// Réplicas A e B partem do mesmo estado
	a := NewAWORSet[string]()
	b := NewAWORSet[string]()
	a.Merge(seed)
	b.Merge(seed)

	// Operações concorrentes…
	a.Add("A", "x") // novo dot ⟶ Add
	b.Remove("x")   // Remove só dots já observados

	// …e propagação
	a.Merge(b)
	b.Merge(a)

	if !(func() bool {
		_, aok := elems(a)["x"]
		_, bok := elems(b)["x"]
		return aok && bok
	}()) {
		t.Fatalf("Add não venceu Remove na presença de concorrência")
	}
}

// -------------------------------------------------------------------------
// 3. Conmutatividade
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
		t.Fatalf("merge não é comutativo – %v vs %v", elems(left), elems(right))
	}
}

// -------------------------------------------------------------------------
// 4. Associatividade
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
		t.Fatalf("merge não é associativo – %v vs %v", elems(left), elems(right))
	}
}

// -------------------------------------------------------------------------
// 5. Idempotência
// -------------------------------------------------------------------------
func TestMergeIdempotent(t *testing.T) {
	s := NewAWORSet[string]()
	s.Add("A", "z")

	before := elems(s)

	s.Merge(s) // merge consigo mesmo

	if !equal(before, elems(s)) {
		t.Fatalf("merge não é idempotente – antes %v, depois %v", before, elems(s))
	}
}

// -------------------------------------------------------------------------
// 6. Testes adicionais de remoção e semânticas
// -------------------------------------------------------------------------

// TestMultipleAdds verifica que múltiplas adições acumulam elementos
func TestMultipleAdds(t *testing.T) {
	s := NewAWORSet[string]()
	s.Add("A", "a")
	s.Add("A", "b")
	got := elems(s)
	want := map[string]struct{}{"a": {}, "b": {}}
	if !equal(got, want) {
		t.Fatalf("esperava %v, obteve %v", want, got)
	}
}

// TestRemoveWinsWithoutConcurrentAdd testa remoção sem adição concorrente
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
		t.Fatalf("esperava 'x' removido, mas ainda presente em A:%v B:%v", elems(a), elems(b))
	}
}

// TestAddWinsOnConcurrentRemoveAndAdd verifica semântica add-vs-remove concorrente
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
		t.Fatalf("esperava 'x' presente após add-vs-remove concorrente, obteve A:%v B:%v", elems(a), elems(b))
	}
}

// TestReAddAfterRemove verifica que reedição depois de remoção funciona
func TestReAddAfterRemove(t *testing.T) {
	s := NewAWORSet[string]()
	s.Add("A", "go")
	s.Remove("go")
	s.Add("A", "go")
	if _, ok := elems(s)["go"]; !ok {
		t.Fatalf("esperava 'go' presente após remoção e nova adição")
	}
}

// TestRemoveNonExistent verifica que remover elemento inexistente não causa erro
func TestRemoveNonExistent(t *testing.T) {
	s := NewAWORSet[string]()
	s.Remove("nope")
	if len(s.Elements()) != 0 {
		t.Fatalf("esperava conjunto vazio após remover elemento inexistente, obteve %v", s.Elements())
	}
}

// TestMultipleElementsRemoveOne verifica remoção seletiva
func TestMultipleElementsRemoveOne(t *testing.T) {
	s := NewAWORSet[string]()
	s.Add("A", "a")
	s.Add("A", "b")
	s.Remove("a")
	got := elems(s)
	want := map[string]struct{}{"b": {}}
	if !equal(got, want) {
		t.Fatalf("esperava %v após remover 'a', obteve %v", want, got)
	}
}
