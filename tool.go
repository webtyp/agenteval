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
	Action  byte   // required: model.ActionRead, ActionCreate, ActionUpdate or ActionDelete
	Returns string // what the tool answers, whatever the arguments
}

// Validate validates the FakeTool specification.
func (ft FakeTool) Validate() error {
	if ft.Name == "" {
		return ErrFakeToolNeedsName
	}
	switch ft.Action {
	case model.ActionRead, model.ActionCreate, model.ActionUpdate, model.ActionDelete:
		return nil
	default:
		return fmt.Errorf("agenteval: FakeTool %q needs an Action (model.ActionRead, ActionCreate, ActionUpdate or ActionDelete)", ft.Name)
	}
}

type fakeToolWrapper struct {
	ft    FakeTool
	calls *[]ToolCall
}

func (f *fakeToolWrapper) Name() string        { return f.ft.Name }
func (f *fakeToolWrapper) Description() string { return f.ft.Description }
func (f *fakeToolWrapper) InputSchema() string { return f.ft.InputSchema }
func (f *fakeToolWrapper) Action() byte        { return f.ft.Action }

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
