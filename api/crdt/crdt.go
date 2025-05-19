package crdt

// Dot identifica unicamente cada operação Add.
type Dot struct {
	NodeID  string
	Counter int
}

// preciso para ser chave de map
func (d Dot) String() string { return d.NodeID + "#" + itoa(d.Counter) }

type AWORSet[E comparable] struct {
	nodeID   string
	counters map[string]int   // versão por nó
	dots     map[Dot]E        // payloads associados a dots
}

// New cria um AWOR-Set vazio para o nó dado.
func New[E comparable](nodeID string) *AWORSet[E] {
	return &AWORSet[E]{
		nodeID:   nodeID,
		counters: make(map[string]int),
		dots:     make(map[Dot]E),
	}
}

// nextDot gera um dot único para este nó.
func (s *AWORSet[E]) nextDot() Dot {
	s.counters[s.nodeID]++
	return Dot{NodeID: s.nodeID, Counter: s.counters[s.nodeID]}
}

// ---------- API pública --------------------------------------------------

// Add insere um elemento e devolve um delta contendo só o novo dot
// (pode ignorar o retorno se estiver usando a versão state-based pura).
func (s *AWORSet[E]) Add(elem E) *AWORSet[E] {
	d := s.nextDot()
	s.dots[d] = elem

	// delta = set contendo apenas esse dot
	delta := New[E](s.nodeID)
	delta.counters[d.NodeID] = d.Counter
	delta.dots[d] = elem
	return delta
}

// Remove elimina todos os dots conhecidos para "elem" e devolve o delta
// que carrega apenas as remoções (=> vazio, porque neste modelo state-based
// remoções são refletidas pela ausência do dot).
func (s *AWORSet[E]) Remove(elem E) *AWORSet[E] {
	delta := New[E](s.nodeID)
	for d, v := range s.dots {
		if v == elem {
			delete(s.dots, d)
			// nada é adicionado a delta; ausência já codifica remoção
		}
	}
	return delta
}

// Elements devolve o conteúdo lógico do set.
func (s *AWORSet[E]) Elements() map[E]struct{} {
	out := make(map[E]struct{})
	for _, v := range s.dots {
		out[v] = struct{}{}
	}
	return out
}

// Contains testa se elem ∈ S.
func (s *AWORSet[E]) Contains(elem E) bool {
	_, ok := s.Elements()[elem]
	return ok
}

// Merge/join – união de dots; counters são maximizados (VV).
func (s *AWORSet[E]) Merge(other *AWORSet[E]) {
	// une versão-vetor
	for n, c := range other.counters {
		if c > s.counters[n] {
			s.counters[n] = c
		}
	}
	// une dots (add wins)
	for d, v := range other.dots {
		s.dots[d] = v
	}
}

// ---------- Utilitário trivial -------------------------------------------
func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	neg := i < 0
	if neg {
		i = -i
	}
	var digits [20]byte
	pos := len(digits)
	for i > 0 {
		pos--
		digits[pos] = byte('0' + i%10)
		i /= 10
	}
	if neg {
		pos--
		digits[pos] = '-'
	}
	return string(digits[pos:])
}