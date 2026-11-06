// Package fsm implements the OSPF neighbor state machine (RFC 2328 §10).
//
// The interface state machine (§9) collapses to "point-to-point, always up" in
// the Phase 2 emulator, so only the neighbor FSM is modelled here. DR/BDR
// election on broadcast segments is deferred with Network LSAs.
package fsm

import "fmt"

// State is a neighbor conversation state (RFC 2328 §10.1).
type State int

const (
	Down State = iota
	Attempt
	Init
	TwoWay
	ExStart
	Exchange
	Loading
	Full
)

func (s State) String() string {
	switch s {
	case Down:
		return "Down"
	case Attempt:
		return "Attempt"
	case Init:
		return "Init"
	case TwoWay:
		return "TwoWay"
	case ExStart:
		return "ExStart"
	case Exchange:
		return "Exchange"
	case Loading:
		return "Loading"
	case Full:
		return "Full"
	default:
		return fmt.Sprintf("State(%d)", int(s))
	}
}

// Event is an input to the neighbor FSM (RFC 2328 §10.2).
type Event int

const (
	HelloReceived   Event = iota
	TwoWayReceived        // our own Router ID seen in the neighbor's Hello
	NegotiationDone       // master/slave and DD seq agreed
	ExchangeDone          // both sides finished sending DD summaries
	LoadingDone           // request list drained
	AdjOK                 // decided to form an adjacency
	SeqNumMismatch
	KillNbr // Dead timer fired / link down
)

func (e Event) String() string {
	switch e {
	case HelloReceived:
		return "HelloReceived"
	case TwoWayReceived:
		return "TwoWayReceived"
	case NegotiationDone:
		return "NegotiationDone"
	case ExchangeDone:
		return "ExchangeDone"
	case LoadingDone:
		return "LoadingDone"
	case AdjOK:
		return "AdjOK"
	case SeqNumMismatch:
		return "SeqNumMismatch"
	case KillNbr:
		return "KillNbr"
	default:
		return fmt.Sprintf("Event(%d)", int(e))
	}
}

// Next returns the state after applying event e to state s. Unhandled
// (state, event) pairs leave the state unchanged, which is the OSPF default.
//
// On a point-to-point link every adjacency is wanted, so TwoWayReceived leads
// straight to ExStart (AdjOK is implicit).
func Next(s State, e Event) State {
	if e == KillNbr {
		return Down
	}
	switch s {
	case Down:
		if e == HelloReceived {
			return Init
		}
	case Init:
		switch e {
		case TwoWayReceived:
			return ExStart // point-to-point: always form the adjacency
		case HelloReceived:
			return Init
		}
	case TwoWay:
		if e == AdjOK {
			return ExStart
		}
	case ExStart:
		if e == NegotiationDone {
			return Exchange
		}
	case Exchange:
		switch e {
		case ExchangeDone:
			return Loading
		case SeqNumMismatch:
			return ExStart
		}
	case Loading:
		switch e {
		case LoadingDone:
			return Full
		case SeqNumMismatch:
			return ExStart
		}
	case Full:
		if e == SeqNumMismatch {
			return ExStart
		}
	}
	return s
}

// IsAdjacent reports whether LSAs may flood over an adjacency in this state
// (RFC 2328 §10: Exchange and later exchange LSAs; Full is fully synced).
func IsAdjacent(s State) bool { return s >= Exchange }
