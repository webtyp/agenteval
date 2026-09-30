//go:build !wasm

package agenteval

import (
	"errors"
	"fmt"

	"webtyp.com/context"
	"webtyp.com/model"
)

var (
	ErrFakeToolNeedsName = errors.New("agenteval: FakeTool needs a Name")
)

// FakeTool is a tool with a fixed answer. Its definition should be the real tool's, so the
// agent sees exactly what it will see in production.
type FakeTool struct {
	Name, Description, InputSchema string
	Action                         model.Action // required: model.Read, model.Create, model.Update, model.Delete or a combination
	Returns                        string       // what the tool answers, whatever the arguments
}

// Validate validates the FakeTool specification.
func (ft FakeTool) Validate() error {
	if ft.Name == "" {
		return ErrFakeToolNeedsName
	}
	if ft.Action == 0 || (ft.Action&^model.AllActions) != 0 {
		return fmt.Errorf("agenteval: FakeTool %q needs an Action (model.Read, model.Create, model.Update, model.Delete or a combination)", ft.Name)
	}
	return nil
}

type fakeToolWrapper struct {
	ft    FakeTool
	calls *[]ToolCall
}

func (f *fakeToolWrapper) Name() string        { return f.ft.Name }
func (f *fakeToolWrapper) Description() string { return f.ft.Description }
func (f *fakeToolWrapper) InputSchema() string { return f.ft.InputSchema }
func (f *fakeToolWrapper) Action() model.Action { return f.ft.Action }

func (f *fakeToolWrapper) Execute(ctx *context.Context, inputJSON string) (string, error) {
	if f.calls != nil {
		*f.calls = append(*f.calls, ToolCall{
			Name:   f.ft.Name,
			Input:  inputJSON,
			Output: f.ft.Returns,
			Action: f.ft.Action,
		})
	}
	return f.ft.Returns, nil
}

func (ft FakeTool) wrap(calls *[]ToolCall) *fakeToolWrapper {
	return &fakeToolWrapper{
		ft:    ft,
		calls: calls,
	}
}
