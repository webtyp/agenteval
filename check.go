//go:build !wasm

package agenteval

import (
	"fmt"
	"strings"

	"webtyp.com/model"
)

// Attempt is what happened in one run of a scenario.
type Attempt struct {
	Question string // Scenario.When
	At       Moment
	Answer   string     // what agent.Run returned
	Err      error      // what agent.Run returned
	Calls    []ToolCall // every tool call, in order
}

type ToolCall struct {
	Name, Input, Output string
	Action              byte
}

// Check judges one attempt.
type Check interface {
	Name() string // how the check reads in the report, e.g. `Contains("18:00")`
	Check(a Attempt, j Judge) Result
}

type Result struct {
	Verdict Verdict
	Reason  string // why, in one line
}

type Verdict int

const (
	Pass Verdict = iota + 1
	Fail
	Unsure
)

// Calls checks that some tool call has the given name.
func Calls(name string) Check {
	return callsCheck{name: name}
}

type callsCheck struct {
	name string
}

func (c callsCheck) Name() string {
	return fmt.Sprintf("Calls(%q)", c.name)
}

func (c callsCheck) Check(a Attempt, j Judge) Result {
	for _, call := range a.Calls {
		if call.Name == c.name {
			return Result{Verdict: Pass, Reason: fmt.Sprintf("tool %q was called", c.name)}
		}
	}
	return Result{Verdict: Fail, Reason: fmt.Sprintf("tool %q was not called", c.name)}
}

// DoesNotModify checks that every tool call has Action == model.ActionRead.
func DoesNotModify() Check {
	return doesNotModifyCheck{}
}

type doesNotModifyCheck struct{}

func (c doesNotModifyCheck) Name() string {
	return "DoesNotModify()"
}

func (c doesNotModifyCheck) Check(a Attempt, j Judge) Result {
	for _, call := range a.Calls {
		if call.Action != model.ActionRead {
			return Result{
				Verdict: Fail,
				Reason:  fmt.Sprintf("tool %q modified state (action %d)", call.Name, call.Action),
			}
		}
	}
	return Result{Verdict: Pass, Reason: "no tool call modified state"}
}

// Contains checks that the answer contains the given substring.
func Contains(text string) Check {
	return containsCheck{text: text}
}

type containsCheck struct {
	text string
}

func (c containsCheck) Name() string {
	return fmt.Sprintf("Contains(%q)", c.text)
}

func (c containsCheck) Check(a Attempt, j Judge) Result {
	if strings.Contains(a.Answer, c.text) {
		return Result{Verdict: Pass, Reason: fmt.Sprintf("the answer contains %q", c.text)}
	}
	return Result{Verdict: Fail, Reason: fmt.Sprintf("the answer does not contain %q", c.text)}
}

// NotContains checks that the answer does not contain the given substring.
func NotContains(text string) Check {
	return notContainsCheck{text: text}
}

type notContainsCheck struct {
	text string
}

func (c notContainsCheck) Name() string {
	return fmt.Sprintf("NotContains(%q)", c.text)
}

func (c notContainsCheck) Check(a Attempt, j Judge) Result {
	if !strings.Contains(a.Answer, c.text) {
		return Result{Verdict: Pass, Reason: fmt.Sprintf("the answer does not contain %q", c.text)}
	}
	return Result{Verdict: Fail, Reason: fmt.Sprintf("the answer contains %q", c.text)}
}
