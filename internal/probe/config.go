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
	phrase := declaredPhrase(len(inst.Commands()), len(inst.Operations()))
	return Result{Name: name, Status: Green, Message: fmt.Sprintf("instance definition loaded (%s declared)", phrase)}
}

// declaredPhrase composes the row's parenthetical by the reference page's
// rule: the commands phrase keeps its three v0 forms, an operations phrase
// joins it with a comma only when there is one to count, and the caller's
// trailing "declared" governs both. Operations are counted and nothing
// more — doctor has no business resetting anyone's database — so the
// count is the whole of what the row says about them, and a definition
// with none reads exactly as it did in v0.
func declaredPhrase(commands, operations int) string {
	var phrase string
	switch commands {
	case 0:
		phrase = "no commands"
	case 1:
		phrase = "1 command"
	default:
		phrase = fmt.Sprintf("%d commands", commands)
	}
	switch operations {
	case 0:
	case 1:
		phrase += ", 1 operation"
	default:
		phrase += fmt.Sprintf(", %d operations", operations)
	}
	return phrase
}
