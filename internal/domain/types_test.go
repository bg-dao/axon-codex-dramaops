package domain

import "testing"

func TestRunTransitions(t *testing.T) {
	valid := [][2]RunStatus{
		{RunQueued, RunAwaitingApproval},
		{RunAwaitingApproval, RunRunning},
		{RunRunning, RunSucceeded},
		{RunRunning, RunFailed},
		{RunRunning, RunCancelled},
	}
	for _, transition := range valid {
		if !CanTransitionRun(transition[0], transition[1]) {
			t.Fatalf("expected transition %s -> %s", transition[0], transition[1])
		}
	}
	for _, terminal := range []RunStatus{RunSucceeded, RunFailed, RunCancelled} {
		if CanTransitionRun(terminal, RunRunning) {
			t.Fatalf("terminal status %s must not transition back to running", terminal)
		}
	}
}
