// Package lsdb is the link-state database: storage, LSA comparison, aging and
// the flooding decision (RFC 2328 §12, §13).
package lsdb

import (
	"sync"
	"time"

	"github.com/babarkhan811/sospf/internal/packet"
)

// Cmp is the result of comparing two instances of the same LSA.
type Cmp int

const (
	Older Cmp = iota - 1 // the candidate is older than what we hold
	Same                 // identical instance
	Newer                // the candidate is more recent
)

// Compare orders two LSA instances of the same (Type, ID, AdvRouter) by the
// rules of RFC 2328 §13.1: sequence number, then checksum, then age (a MaxAge
// instance, or one at least MaxAgeDiff newer, is considered more recent).
func Compare(a, b packet.LSAHeader) Cmp {
	if a.SeqNum != b.SeqNum {
		if a.SeqNum > b.SeqNum {
			return Newer
		}
		return Older
	}
	if a.Checksum != b.Checksum {
		if a.Checksum > b.Checksum {
			return Newer
		}
		return Older
	}
	if a.Age == packet.MaxAge && b.Age != packet.MaxAge {
		return Newer
	}
	if b.Age == packet.MaxAge && a.Age != packet.MaxAge {
		return Older
	}
	diff := int(a.Age) - int(b.Age)
	if diff < 0 {
		diff = -diff
	}
	if uint16(diff) > packet.MaxAgeDiff {
		if a.Age < b.Age {
			return Newer
		}
		return Older
	}
	return Same
}

type entry struct {
	lsa     *packet.RouterLSA
	arrived time.Time // for age accounting
}

// DB is a link-state database. It is safe for concurrent use.
type DB struct {
	mu      sync.RWMutex
	entries map[packet.Key]*entry
}

// New returns an empty database.
func New() *DB { return &DB{entries: make(map[packet.Key]*entry)} }

// Install stores lsa if it is newer than (or absent from) the database. It
// reports whether the database changed, which the caller uses to decide whether
// to flood the LSA onward and recompute SPF.
func (d *DB) Install(lsa *packet.RouterLSA, now time.Time) (changed bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	k := lsa.Header.Key()
	cur, ok := d.entries[k]
	if ok && Compare(lsa.Header, cur.lsa.Header) != Newer {
		return false
	}
	d.entries[k] = &entry{lsa: lsa, arrived: now}
	return true
}

// Get returns the stored LSA for k, or nil.
func (d *DB) Get(k packet.Key) *packet.RouterLSA {
	d.mu.RLock()
	defer d.mu.RUnlock()
	if e, ok := d.entries[k]; ok {
		return e.lsa
	}
	return nil
}

// Has reports whether the database holds an instance at least as recent as h.
func (d *DB) Has(h packet.LSAHeader) bool {
	d.mu.RLock()
	defer d.mu.RUnlock()
	e, ok := d.entries[h.Key()]
	if !ok {
		return false
	}
	return Compare(h, e.lsa.Header) != Newer
}

// All returns a snapshot of every stored Router LSA, with each LSA's Age field
// advanced to reflect time held in the database (capped at MaxAge).
func (d *DB) All(now time.Time) []*packet.RouterLSA {
	d.mu.RLock()
	defer d.mu.RUnlock()
	out := make([]*packet.RouterLSA, 0, len(d.entries))
	for _, e := range d.entries {
		cp := *e.lsa
		cp.Header.Age = agedValue(e, now)
		out = append(out, &cp)
	}
	return out
}

// Len returns the number of LSAs in the database.
func (d *DB) Len() int {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return len(d.entries)
}

func agedValue(e *entry, now time.Time) uint16 {
	age := int(e.lsa.Header.Age) + int(now.Sub(e.arrived).Seconds())
	if age > int(packet.MaxAge) {
		return packet.MaxAge
	}
	return uint16(age)
}
