package ledger

import "testing"

// used returns a toolhead's UsedMM from a fresh snapshot.
func used(l *Ledger, index int) float64 {
	for _, t := range l.Snapshot() {
		if t.Index == index {
			return t.UsedMM
		}
	}
	return -1
}

// TestAttributionAcrossToolChange is the heart of the multi-color fix: the U1
// reports one cumulative filament counter, and each delta must be charged to
// whichever tool was active when it advanced — not all to one spool.
func TestAttributionAcrossToolChange(t *testing.T) {
	l := New(4)

	// T0 prints first. First observation only sets the baseline (no delta yet).
	l.SetActive(0)
	l.ObserveTotalFilament(1000)
	if got := used(l, 0); got != 0 {
		t.Fatalf("first observation should set baseline, got T0 used=%v", got)
	}
	l.ObserveTotalFilament(1100) // +100 on T0
	if got := used(l, 0); got != 100 {
		t.Fatalf("T0 used = %v, want 100", got)
	}

	// Tool change to T1; the counter keeps climbing. New consumption is T1's.
	l.SetActive(1)
	l.ObserveTotalFilament(1250) // +150 on T1
	if got := used(l, 1); got != 150 {
		t.Fatalf("T1 used = %v, want 150", got)
	}

	// Back to T0; its tally resumes and accumulates on top of its earlier 100.
	l.SetActive(0)
	l.ObserveTotalFilament(1300) // +50 on T0
	if got := used(l, 0); got != 150 {
		t.Fatalf("T0 used = %v, want 150 (100 + 50)", got)
	}

	// T1 must be untouched by T0's later printing.
	if got := used(l, 1); got != 150 {
		t.Fatalf("T1 used = %v, want 150 (unchanged)", got)
	}

	// Total charged across tools equals total filament consumed since baseline.
	if total := used(l, 0) + used(l, 1); total != 300 {
		t.Fatalf("total attributed = %v, want 300", total)
	}
}

// TestNonAdvancingCounterIgnored guards against retractions / counter resets
// inflating usage: only positive deltas count.
func TestNonAdvancingCounterIgnored(t *testing.T) {
	l := New(2)
	l.SetActive(0)
	l.ObserveTotalFilament(500) // baseline
	l.ObserveTotalFilament(600) // +100
	l.ObserveTotalFilament(580) // retraction: ignored
	l.ObserveTotalFilament(610) // +10 from the last seen high? no — delta vs 580

	// After 580 (stored) then 610, delta = +30. Total = 100 + 30 = 130.
	if got := used(l, 0); got != 130 {
		t.Fatalf("T0 used = %v, want 130", got)
	}
}

// TestSetTempPartial verifies temp/target update independently, since Moonraker
// streams only changed fields.
func TestSetTempPartial(t *testing.T) {
	l := New(1)
	temp, target := 215.0, 220.0
	l.SetTemp(0, &temp, &target)
	if got := l.Snapshot()[0]; got.Temperature != 215 || got.Target != 220 {
		t.Fatalf("got temp=%v target=%v, want 215/220", got.Temperature, got.Target)
	}
	// A frame with only temperature must not zero the target.
	newTemp := 50.0
	l.SetTemp(0, &newTemp, nil)
	if got := l.Snapshot()[0]; got.Temperature != 50 || got.Target != 220 {
		t.Fatalf("got temp=%v target=%v, want 50/220 (target unchanged)", got.Temperature, got.Target)
	}
}
