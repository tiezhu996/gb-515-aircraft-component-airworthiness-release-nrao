package constants

import "testing"

func TestAircraftPartTransitionGraph(t *testing.T) {
	if !CanTransition(AircraftPartTransitions, "received", "inspection") {
		t.Fatalf("expected received -> inspection transition to be allowed")
	}
	if CanTransition(AircraftPartTransitions, "received", "unknown") {
		t.Fatal("unknown status must never be accepted")
	}
}
