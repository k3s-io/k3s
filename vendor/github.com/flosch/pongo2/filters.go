package pongo2

import (
	"fmt"

	"github.com/juju/errors"
)

// FilterFunction is the type filter functions must fulfil
type FilterFunction func(in *Value, param *Value) (out *Value, err *Error)

var filters map[string]FilterFunction

func init() {
	filters = make(map[string]FilterFunction)
}

// FilterExists returns true if the given filter is already registered
func FilterExists(name string) bool {
	_, existing := filters[name]
	return existing
}

// RegisterFilter registers a new filter. If there's already a filter with the same
// name, RegisterFilter will panic. You usually want to call this
// function in the filter's init() function:
// http://golang.org/doc/effective_go.html#init
//
// See http://www.florian-schlachter.de/post/pongo2/ for more about
// writing filters and tags.
func RegisterFilter(name string, fn FilterFunction) error {
	if FilterExists(name) {
		return errors.Errorf("filter with name '%s' is already registered", name)
	}
	filters[name] = fn
	return nil
}

// ReplaceFilter replaces an already registered filter with a new implementation. Use this
// function with caution since it allows you to change existing filter behaviour.
func ReplaceFilter(name string, fn FilterFunction) error {
	if !FilterExists(name) {
		return errors.Errorf("filter with name '%s' does not exist (therefore cannot be overridden)", name)
	}
	filters[name] = fn
	return nil
}

// MustApplyFilter behaves like ApplyFilter, but panics on an error.
func MustApplyFilter(name string, value *Value, param *Value) *Value {
	val, err := ApplyFilter(name, value, param)
	if err != nil {
		panic(err)
	}
	return val
}

// ApplyFilter applies a filter to a given value using the given parameters.
// Returns a *pongo2.Value or an error.
func ApplyFilter(name string, value *Value, param *Value) (*Value, *Error) {
	fn, existing := filters[name]
	if !existing {
		return nil, &Error{
			Sender:    "applyfilter",
			OrigError: errors.Errorf("Filter with name '%s' not found.", name),
		}
	}

	// Make sure param is a *Value
	if param == nil {
		param = AsValue(nil)
	}

	return fn(value, param)
}

type filterCall struct {
	token *Token

	name      string
	parameter IEvaluator

	filterFunc FilterFunction
}

func (fc *filterCall) Execute(v *Value, ctx *ExecutionContext) (*Value, *Error) {
	var param *Value
	var err *Error

	if fc.parameter != nil {
		param, err = fc.parameter.Evaluate(ctx)
		if err != nil {
			return nil, err
		}
	} else {
		param = AsValue(nil)
	}

	filteredValue, err := fc.filterFunc(v, param)
	if err != nil {
		return nil, err.updateFromTokenIfNeeded(ctx.template, fc.token)
	}
	return filteredValue, nil
}

// Filter = IDENT | IDENT ":" FilterArg | IDENT "|" Filter
func (p *Parser) parseFilter() (*filterCall, *Error) {
	identToken := p.MatchType(TokenIdentifier)

	// Check filter ident
	if identToken == nil {
		return nil, p.Error("Filter name must be an identifier.", nil)
	}

	filter := &filterCall{
		token: identToken,
		name:  identToken.Val,
	}

	// Get the appropriate filter function and bind it
	filterFn, exists := filters[identToken.Val]
	if !exists {
		return nil, p.Error(fmt.Sprintf("Filter '%s' does not exist.", identToken.Val), identToken)
	}

	filter.filterFunc = filterFn

	// Check for filter-argument (2 tokens needed: ':' ARG)
	if p.Match(TokenSymbol, ":") != nil {
		if p.Peek(TokenSymbol, "}}") != nil {
			return nil, p.Error("Filter parameter required after ':'.", nil)
		}

		// Get filter argument expression
		v, err := p.parseVariableOrLiteral()
		if err != nil {
			return nil, err
		}
		filter.parameter = v
	}

	return filter, nil
}
