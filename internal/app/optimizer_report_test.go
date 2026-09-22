package app

import "testing"

func TestPublicOptimizationMode(t *testing.T) {
	tests := []struct {
		plan searchPlan
		want string
	}{
		{plan: searchPlan{solverMode: "score"}, want: "score"},
		{plan: searchPlan{solverMode: "score", approximate: true}, want: "fast-score"},
		{plan: searchPlan{solverMode: "objective", approximate: true}, want: "fast"},
	}
	for _, test := range tests {
		if got := publicOptimizationMode(test.plan); got != test.want {
			t.Fatalf("publicOptimizationMode(%#v) = %q, want %q", test.plan, got, test.want)
		}
	}
}
