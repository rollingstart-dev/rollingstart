package probe

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/rollingstart-dev/rollingstart/internal/instance"
)

// InstanceConfig probes whether the instance definition at dir loads. The
// loader's distinct failures — missing, unparseable, invalid — carry
// self-naming, positioned messages, and they surface here verbatim so the
// finding points at the actual problem. The loader's Detail() source
// excerpts are deliberately not folded into Message; whether rendering
// wants them is #4's call, against the ParseError type directly. ctx keeps
// the probe signatures uniform; loading is one local file read with
// nothing to cancel.
func InstanceConfig(ctx context.Context, dir string) Result {
	const name = "instance definition"
	inst, err := instance.Load(filepath.Join(dir, instance.Path))
	if err != nil {
		return Result{Name: name, Status: Red, Message: err.Error()}
	}
	switch n := len(inst.Commands()); n {
	case 0:
		return Result{Name: name, Status: Green, Message: "instance definition loaded (no commands declared)"}
	case 1:
		return Result{Name: name, Status: Green, Message: "instance definition loaded (1 command declared)"}
	default:
		return Result{Name: name, Status: Green, Message: fmt.Sprintf("instance definition loaded (%d commands declared)", n)}
	}
}
