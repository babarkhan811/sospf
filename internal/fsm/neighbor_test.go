package fsm

import "testing"

func TestHappyPathToFull(t *testing.T) {
	seq := []struct {
		ev   Event
		want State
	}{
		{HelloReceived, Init},
		{TwoWayReceived, ExStart},
		{NegotiationDone, Exchange},
		{ExchangeDone, Loading},
		{LoadingDone, Full},
	}
	s := Down
	for i, step := range seq {
		s = Next(s, step.ev)
		if s != step.want {
			t.Fatalf("step %d: after %v got %v want %v", i, step.ev, s, step.want)
		}
	}
}

func TestKillFromAnyState(t *testing.T) {
	for s := Down; s <= Full; s++ {
		if got := Next(s, KillNbr); got != Down {
			t.Errorf("KillNbr from %v: got %v want Down", s, got)
		}
	}
}

func TestSeqNumMismatchRestartsExchange(t *testing.T) {
	for _, s := range []State{Exchange, Loading, Full} {
		if got := Next(s, SeqNumMismatch); got != ExStart {
			t.Errorf("SeqNumMismatch from %v: got %v want ExStart", s, got)
		}
	}
}

func TestUnhandledEventsAreNoOps(t *testing.T) {
	if got := Next(Full, HelloReceived); got != Full {
		t.Errorf("Hello in Full should stay Full, got %v", got)
	}
	if got := Next(Down, LoadingDone); got != Down {
		t.Errorf("LoadingDone in Down should stay Down, got %v", got)
	}
}

func TestIsAdjacent(t *testing.T) {
	if IsAdjacent(Init) || IsAdjacent(TwoWay) {
		t.Error("Init/TwoWay are not adjacent for flooding")
	}
	if !IsAdjacent(Exchange) || !IsAdjacent(Full) {
		t.Error("Exchange and Full must be adjacent for flooding")
	}
}
