package main

import (
	"slices"
	"testing"
)

func TestReviewSpecChangeIsLimitedToProposedAcquisitionCandidate(t *testing.T) {
	runner := commandRunner{
		platform:         "linux/amd64",
		repoRoot:         "/repo",
		reviewSpecChange: true,
	}
	pinned := runner.candidateArgs("", "/tmp/pinned")
	if slices.Contains(pinned, "-review-spec-change") {
		t.Fatalf("pinned candidate arguments relaxed compatibility assertions: %v", pinned)
	}
	proposed := runner.candidateArgs("/tmp/proposal", "/tmp/proposed")
	if !slices.Contains(proposed, "-review-spec-change") {
		t.Fatalf("proposed review candidate arguments lack review mode: %v", proposed)
	}

	strict := runner
	strict.reviewSpecChange = false
	finalized := strict.candidateArgs("/tmp/final", "/tmp/candidate")
	if slices.Contains(finalized, "-review-spec-change") {
		t.Fatalf("finalized candidate arguments relaxed compatibility assertions: %v", finalized)
	}
}
