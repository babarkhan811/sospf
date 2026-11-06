package trust

import (
	"testing"
	"time"

	"github.com/babarkhan811/sospf/internal/semantic"
)

func agent(s string) semantic.AgentID {
	var id semantic.AgentID
	copy(id[:], s)
	return id
}

func TestNewAgentStartsAtFloor(t *testing.T) {
	m := New(DefaultParams())
	if got := m.Score(agent("new")); got != 0.5 {
		t.Errorf("new agent trust: got %.3f want 0.5 (floor)", got)
	}
}

func TestEWMAConvergesUpOnSuccess(t *testing.T) {
	m := New(DefaultParams())
	id := agent("good")
	var last float64
	for i := 0; i < 50; i++ {
		last = m.Update(id, Outcome{Success: true, SLAAdhered: true})
	}
	if last < 0.95 {
		t.Errorf("sustained success should drive trust high, got %.3f", last)
	}
}

func TestAnomalyDrivesTrustDown(t *testing.T) {
	m := New(DefaultParams())
	id := agent("bad")
	for i := 0; i < 5; i++ {
		m.Update(id, Outcome{Success: true, SLAAdhered: true})
	}
	high := m.Score(id)
	for i := 0; i < 20; i++ {
		m.Update(id, Outcome{Success: false, Anomaly: true})
	}
	low := m.Score(id)
	if low >= high {
		t.Errorf("anomalies should lower trust: high=%.3f low=%.3f", high, low)
	}
}

func TestDecayTowardFloor(t *testing.T) {
	p := DefaultParams()
	p.DecayHalfLife = time.Minute
	m := New(p)
	now := time.Unix(0, 0)
	m.now = func() time.Time { return now }
	id := agent("idle")
	for i := 0; i < 50; i++ {
		m.Update(id, Outcome{Success: true, SLAAdhered: true})
	}
	hi := m.Score(id)
	now = now.Add(3 * time.Minute) // 3 half-lives
	decayed := m.Score(id)
	if decayed >= hi || decayed <= p.NewAgentFloor {
		t.Errorf("trust should decay toward (not below) the floor: hi=%.3f decayed=%.3f floor=%.3f", hi, decayed, p.NewAgentFloor)
	}
}

func TestReOriginationThreshold(t *testing.T) {
	m := New(DefaultParams())
	id := agent("x")
	m.Update(id, Outcome{Success: true, SLAAdhered: true}) // moves well past 0.05 from floor
	if _, need := m.NeedsReOrigination(id); !need {
		t.Error("a large trust move should require re-origination")
	}
	if _, need := m.NeedsReOrigination(id); need {
		t.Error("immediately after advertising, no re-origination is needed")
	}
}

func TestDamperSuppressesNoiseButPassesSustainedChange(t *testing.T) {
	d := NewDamper(0.3, 10.0) // band of 10 units
	d.Observe(100)            // prime
	// Small +/- noise around 100 must not report.
	flaps := 0
	for _, s := range []float64{102, 98, 101, 99, 103, 97} {
		if _, changed := d.Observe(s); changed {
			flaps++
		}
	}
	if flaps != 0 {
		t.Errorf("noise within the band should not report changes, got %d", flaps)
	}
	// A sustained jump to ~160 must eventually report.
	reportedJump := false
	for i := 0; i < 30; i++ {
		if _, changed := d.Observe(160); changed {
			reportedJump = true
			break
		}
	}
	if !reportedJump {
		t.Error("a sustained large change should report")
	}
}

func TestTrustLSARoundTrip(t *testing.T) {
	in := &semantic.TrustLSA{
		AdvRouter: 0x01010101, OpaqueID: 3, Seq: -0x7FFFFFFF,
		Subject: agent("subject-1"), Score: semantic.ScoreFromFloat(0.82),
		Scope: semantic.TrustScopeOverall, EvidenceAlg: 1, EvidenceDigest: []byte{1, 2, 3},
		NotBefore: 1000, NotAfter: 2000,
	}
	out, err := semantic.DecodeTrustLSA(in.Encode())
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out.Subject != in.Subject || out.Score != in.Score || out.NotAfter != 2000 {
		t.Errorf("trust LSA round trip mismatch: %+v", out)
	}
	if got := semantic.FloatFromScore(out.Score); got < 0.81 || got > 0.83 {
		t.Errorf("trust score conversion off: %.3f", got)
	}
}

func TestTelemetryLSARoundTrip(t *testing.T) {
	in := &semantic.TelemetryLSA{
		AdvRouter: 1, OpaqueID: 2, Seq: -0x7FFFFFFF,
		Subject: agent("a"), HasSubject: true,
		LatencyUs: 20000, JitterUs: 500, SuccessRate: semantic.ScoreFromFloat(0.99),
		Load: semantic.ScoreFromFloat(0.25), QueueDepth: 4, Cost: 7, Energy: 12,
	}
	out, err := semantic.DecodeTelemetryLSA(in.Encode())
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out.LatencyUs != 20000 || out.QueueDepth != 4 || !out.HasSubject {
		t.Errorf("telemetry LSA round trip mismatch: %+v", out)
	}
}
