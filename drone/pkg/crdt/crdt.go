package crdt

import (
	"encoding/json"
	"fmt"
)

// Dot uniquely identifies each operation on a replica.
type Dot struct {
	NodeID  string `json:"node_id"`
	Counter int64  `json:"counter"`
}

// String provides a map key for Dot.
func (d Dot) String() string {
	return fmt.Sprintf("%s#%d", d.NodeID, d.Counter)
}

// VectorClock holds the highest continuous counter per node.
type VectorClock map[string]int64

// DotCloud holds all dots that fall outside the continuous prefix.
// Using bool instead of struct{} for JSON serialization compatibility
type DotCloud map[Dot]bool

// DotContext combines a VectorClock and a DotCloud,
// and can be compacted to keep metadata minimal.
type DotContext struct {
	Clock    VectorClock
	DotCloud DotCloud
}

// MarshalJSON converts the internal DotCloud map into a slice for JSON.
func (ctx DotContext) MarshalJSON() ([]byte, error) {
	cloud := make([]Dot, 0, len(ctx.DotCloud))
	for d := range ctx.DotCloud {
		cloud = append(cloud, d)
	}
	alias := struct {
		Clock    VectorClock `json:"clock"`
		DotCloud []Dot       `json:"dot_cloud"`
	}{
		Clock:    ctx.Clock,
		DotCloud: cloud,
	}
	return json.Marshal(alias)
}

// UnmarshalJSON rebuilds DotCloud map from a slice.
func (ctx *DotContext) UnmarshalJSON(data []byte) error {
	alias := struct {
		Clock    VectorClock `json:"clock"`
		DotCloud []Dot       `json:"dot_cloud"`
	}{
		Clock:    make(VectorClock),
		DotCloud: nil,
	}
	if err := json.Unmarshal(data, &alias); err != nil {
		return err
	}
	ctx.Clock = alias.Clock
	ctx.DotCloud = make(DotCloud, len(alias.DotCloud))
	for _, d := range alias.DotCloud {
		ctx.DotCloud[d] = true
	}
	return nil
}

// NewDotContext creates an empty DotContext.
func NewDotContext() *DotContext {
	return &DotContext{
		Clock:    make(VectorClock),
		DotCloud: make(DotCloud),
	}
}

// Contains returns true if the dot is known in either the clock or the cloud.
func (ctx *DotContext) Contains(d Dot) bool {
	if v, ok := ctx.Clock[d.NodeID]; ok && v >= d.Counter {
		return true
	}
	_, inCloud := ctx.DotCloud[d]
	return inCloud
}

// NextDot advances the local clock for nodeID and returns a fresh dot.
func (ctx *DotContext) NextDot(nodeID string) Dot {
	cur := ctx.Clock[nodeID]
	next := cur + 1
	ctx.Clock[nodeID] = next
	return Dot{NodeID: nodeID, Counter: next}
}

// Merge combines another DotContext into this one and compacts.
func (ctx *DotContext) Merge(other *DotContext) {
	// 1) Merge clocks: take max per node
	for n, c := range other.Clock {
		if c > ctx.Clock[n] {
			ctx.Clock[n] = c
		}
	}
	// 2) Merge cloud
	for d := range other.DotCloud {
		ctx.DotCloud[d] = true
	}
	// 3) Compact to remove now-contiguous or obsolete dots
	ctx.compact()
}

// compact folds any dots that are now continuous into the clock,
// and removes any that are already covered by the clock.
func (ctx *DotContext) compact() {
	var toRemove []Dot
	for d := range ctx.DotCloud {
		maxCont := ctx.Clock[d.NodeID]
		switch {
		case d.Counter == maxCont+1:
			// This dot continues the prefix
			ctx.Clock[d.NodeID] = d.Counter
			toRemove = append(toRemove, d)
		case d.Counter <= maxCont:
			// Already represented in the clock
			toRemove = append(toRemove, d)
		}
	}
	for _, d := range toRemove {
		delete(ctx.DotCloud, d)
	}
}

// -----------------------------------------------------------------------
// DotKernel holds only the active entries (no tombstones) plus a DotContext.
type DotKernel[E comparable] struct {
	Context *DotContext
	Entries map[Dot]E
}

// NewDotKernel creates an empty DotKernel.
func NewDotKernel[E comparable]() *DotKernel[E] {
	return &DotKernel[E]{
		Context: NewDotContext(),
		Entries: make(map[Dot]E),
	}
}

// Values returns a slice of all active elements.
func (k *DotKernel[E]) Values() []E {
	vals := make([]E, 0, len(k.Entries))
	for _, v := range k.Entries {
		vals = append(vals, v)
	}
	return vals
}

// Merge incorporates another kernel: adds unseen entries and removes
// entries that the other context has seen but are not in other.Entries.
func (k *DotKernel[E]) Merge(other *DotKernel[E]) {
	// 1) Add entries unseen by this kernel
	for d, v := range other.Entries {
		if _, seen := k.Entries[d]; !seen && !k.Context.Contains(d) {
			k.Entries[d] = v
		}
	}
	// 2) Remove entries that the other context knows but are not in other.Entries
	for d := range k.Entries {
		if other.Context.Contains(d) {
			if _, stillPresent := other.Entries[d]; !stillPresent {
				delete(k.Entries, d)
			}
		}
	}
	// 3) Merge contexts
	k.Context.Merge(other.Context)
}

// -----------------------------------------------------------------------
// AWORSet is an Add-Wins Observed-Remove Set with delta-awareness.
type AWORSet[E comparable] struct {
	Core  *DotKernel[E]
	Delta *DotKernel[E] // nil if no pending updates
}

// NewAWORSet creates an empty set.
func NewAWORSet[E comparable]() *AWORSet[E] {
	return &AWORSet[E]{
		Core:  NewDotKernel[E](),
		Delta: nil,
	}
}

// Add inserts v and records the operation in Delta.
func (s *AWORSet[E]) Add(nodeID string, v E) {
	if s.Delta == nil {
		s.Delta = NewDotKernel[E]()
	}
	// Generate a new dot in Core's context
	d := s.Core.Context.NextDot(nodeID)
	// Add to Core and Delta
	s.Core.Entries[d] = v
	s.Delta.Entries[d] = v
}

// Remove deletes all occurrences of v and marks removals in Delta.Context.
func (s *AWORSet[E]) Remove(v E) {
	if s.Delta == nil {
		s.Delta = NewDotKernel[E]()
	}
	for d, vv := range s.Core.Entries {
		if vv == v {
			delete(s.Core.Entries, d)
			// Mark the removal in Delta's context
			s.Delta.Context.DotCloud[d] = true
		}
	}
	// Compact Delta's context to avoid unbounded cloud growth
	s.Delta.Context.compact()
}

// MergeDelta applies a received delta kernel into Core and clears Delta.
func (s *AWORSet[E]) MergeDelta(delta *DotKernel[E]) {
	s.Core.Merge(delta)
	s.Delta = nil
}

// Merge incorporates another full AWORSet (state-based merge).
func (s *AWORSet[E]) Merge(other *AWORSet[E]) {
	s.Core.Merge(other.Core)
	if other.Delta != nil {
		s.MergeDelta(other.Delta)
	}
}

// Elements returns the current active elements in the set.
func (s *AWORSet[E]) Elements() []E {
	return s.Core.Values()
}
