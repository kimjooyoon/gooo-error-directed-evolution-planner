package planner

import "testing"

func TestStatusPrecedence(t *testing.T) {
	if got := reduceStatus(DecisionClosed, DecisionUnknown, DecisionRefuted); got != DecisionRefuted {
		t.Fatalf("precedence = %s", got)
	}
	if got := reduceStatus(DecisionClosed, DecisionUnknown); got != DecisionUnknown {
		t.Fatalf("precedence = %s", got)
	}
}

func TestEqualMinimumCandidatesRemainUnknown(t *testing.T) {
	optionA := CandidateOption{ID: "a", Kind: "ADD", Cost: 1, Decision: DecisionClosed}
	optionB := CandidateOption{ID: "b", Kind: "REWIRE", Cost: 1, Decision: DecisionClosed}
	selected, status, record := selectCandidate([]CandidateOption{optionA, optionB})
	if selected != nil || status != DecisionUnknown || record == nil || record.UnknownClass != "AMBIGUOUS_EQUAL_CANDIDATES" || !record.valid() {
		t.Fatalf("unexpected tie result: selected=%v status=%s record=%+v", selected, status, record)
	}
}

func TestMinimumCandidateIsDeterministic(t *testing.T) {
	optionA := CandidateOption{ID: "split", Kind: "SPLIT", Cost: 1, Decision: DecisionClosed}
	optionB := CandidateOption{ID: "add", Kind: "ADD", Cost: 2, Decision: DecisionClosed}
	selected, status, record := selectCandidate([]CandidateOption{optionB, optionA})
	if selected == nil || selected.Kind != "SPLIT" || status != DecisionClosed || record != nil {
		t.Fatalf("unexpected selected result: selected=%v status=%s record=%+v", selected, status, record)
	}
}

func TestUnknownTupleIsComplete(t *testing.T) {
	record := unknown("STAGE", "STEP", "reason", "CLASS", "NEXT", "blocked")
	if !record.valid() {
		t.Fatal("unknown tuple is incomplete")
	}
}

