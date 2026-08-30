package probe

import "testing"

func TestZeroResultIsNotGreen(t *testing.T) {
	var res Result
	if res.Status == Green {
		t.Error("zero Result reads as Green — an unset finding would render as passing")
	}
}
