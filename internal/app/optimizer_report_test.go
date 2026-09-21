package app

import "testing"

func TestPublicOptimizationMode(t *testing.T) {
	tests := []struct {
		plan searchPlan
		want string
	}{
		{plan: searchPlan{solverMode: "score"}, want: "score"},
		{plan: searchPlan{solverMode: "score", approximate: true}, want: "fast-score"},
		{plan: searchPlan{solverMode: "balanced", approximate: true}, want: "fast-balanced"},
		{plan: searchPlan{solverMode: "balanced", approximate: true, compromise: true}, want: "compromise"},
	}
	for _, test := range tests {
		if got := publicOptimizationMode(test.plan); got != test.want {
			t.Fatalf("publicOptimizationMode(%#v) = %q, want %q", test.plan, got, test.want)
		}
	}
}
