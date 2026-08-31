package doctor

import (
	"testing"

	"github.com/rollingstart-dev/rollingstart/internal/probe"
)

func TestBlocking(t *testing.T) {
	green := probe.Result{Name: "a", Status: probe.Green, Message: "fine"}
	red := probe.Result{Name: "b", Status: probe.Red, Message: "broken"}
	tests := []struct {
		name    string
		harness []probe.Result
		want    bool
	}{
		{"no probes ran", nil, true},
		{"all green", []probe.Result{green, green}, false},
		{"one red", []probe.Result{green, red, green}, true},
		{"a status never set", []probe.Result{green, {Name: "c"}}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// The instance section never affects blocking: red there is
			// informational (roadmap § 2.6).
			r := Report{Harness: tt.harness, Instance: Skipped("whatever")}
			if got := r.Blocking(); got != tt.want {
				t.Errorf("Blocking() = %v, want %v", got, tt.want)
			}
		})
	}
}
