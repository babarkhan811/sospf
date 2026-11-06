// Package sft is the Semantic Forwarding Table: the local, computed result of
// intent resolution (WIRE_FORMAT §6). It is never flooded. Entries carry an
// expiry because "a decision is not a capacity reservation; expired decisions
// must not be used" (draft-cui §6.7).
package sft

import (
	"sort"
	"sync"
	"time"

	"github.com/babarkhan811/sospf/internal/semantic"
)

// Key indexes the table by intent category and semantic prefix.
type Key struct {
	IntentCategory string
	Prefix         string
}

// Entry is one resolved forwarding decision.
type Entry struct {
	NextHop     uint32
	OutIface    string
	TargetAgent semantic.AgentID
	Cost        float64
	Constraints semantic.Constraints
	ExpiresAt   time.Time
}

// Table is a concurrency-safe Semantic Forwarding Table.
type Table struct {
	mu  sync.RWMutex
	m   map[Key]Entry
	now func() time.Time
}

// New returns an empty table.
func New() *Table { return &Table{m: make(map[Key]Entry), now: time.Now} }

// Put inserts or replaces an entry.
func (t *Table) Put(k Key, e Entry) {
	t.mu.Lock()
	t.m[k] = e
	t.mu.Unlock()
}

// Lookup returns a live (non-expired) entry for k.
func (t *Table) Lookup(k Key) (Entry, bool) {
	t.mu.RLock()
	e, ok := t.m[k]
	t.mu.RUnlock()
	if !ok || !e.ExpiresAt.IsZero() && t.now().After(e.ExpiresAt) {
		return Entry{}, false
	}
	return e, true
}

// Sweep removes expired entries and returns how many were dropped.
func (t *Table) Sweep() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	now := t.now()
	n := 0
	for k, e := range t.m {
		if !e.ExpiresAt.IsZero() && now.After(e.ExpiresAt) {
			delete(t.m, k)
			n++
		}
	}
	return n
}

// InstallDecision records a resolver Decision for (category, prefix). It returns
// false if the decision selected nothing.
func (t *Table) InstallDecision(category, prefix string, iv semantic.IntentVector, d semantic.Decision) bool {
	if d.Selected == nil {
		return false
	}
	t.Put(Key{IntentCategory: category, Prefix: prefix}, Entry{
		NextHop:     d.Selected.NextHop,
		TargetAgent: d.Selected.Cap.AgentID,
		Cost:        d.Cost,
		Constraints: iv.Constraints,
		ExpiresAt:   d.ExpiresAt,
	})
	return true
}

// Keys returns the table's keys in a deterministic order.
func (t *Table) Keys() []Key {
	t.mu.RLock()
	defer t.mu.RUnlock()
	ks := make([]Key, 0, len(t.m))
	for k := range t.m {
		ks = append(ks, k)
	}
	sort.Slice(ks, func(i, j int) bool {
		if ks[i].IntentCategory != ks[j].IntentCategory {
			return ks[i].IntentCategory < ks[j].IntentCategory
		}
		return ks[i].Prefix < ks[j].Prefix
	})
	return ks
}

// Len returns the number of entries.
func (t *Table) Len() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return len(t.m)
}
