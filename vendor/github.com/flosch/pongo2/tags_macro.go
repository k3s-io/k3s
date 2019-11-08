package pongo2

import (
	"bytes"
	"fmt"
)

type tagMacroNode struct {
	position  *Token
	name      string
	argsOrder []string
	args      map[string]IEvaluator
	exported  bool

	wrapper *NodeWrapper
}

func (node *tagMacroNode) Execute(ctx *ExecutionContext, writer TemplateWriter) *Error {
	ctx.Private[node.name] = func(args ...*Value) *Value {
		return node.call(ctx, args...)
	}

	return nil
}

func (node *tagMacroNode) call(ctx *ExecutionContext, args ...*Value) *Value {
	argsCtx := make(Context)

	for k, v := range node.args {
		if v == nil {
			// User did not provided a default value
			argsCtx[k] = nil
		} else {
			// Evaluate the default value
			valueExpr, err := v.Evaluate(ctx)
			if err != nil {
				ctx.Logf(err.Error())
				return AsSafeValue(err.Error())
			}

			argsCtx[k] = valueExpr
		}
	}

	if len(args) > len(node.argsOrder) {
		// Too many arguments, we're ignoring them and just logging into debug mode.
		err := ctx.Error(fmt.Sprintf("Macro '%s' called with too many arguments (%d instead of %d).",
			node.name, len(args), len(node.argsOrder)), nil).updateFromTokenIfNeeded(ctx.template, node.position)

		ctx.Logf(err.Error()) // TODO: This is a workaround, because the error is not returned yet to the Execution()-methods
		return AsSafeValue(err.Error())
	}

	// Make a context for the macro execution
	macroCtx := NewChildExecutionContext(ctx)

	// Register all arguments in the private context
	macroCtx.Private.Update(argsCtx)

	for idx, argValue := range args {
		macroCtx.Private[node.argsOrder[idx]] = argValue.Interface()
	}

	var b bytes.Buffer
	err := node.wrapper.Execute(macroCtx, &b)
	if err != nil {
		return AsSafeValue(err.updateFromTokenIfNeeded(ctx.template, node.position).Error())
	}

	return AsSafeValue(b.String())
}

func tagMacroParser(doc *Parser, start *Token, arguments *Parser) (INodeTag, *Error) {
	macroNode := &tagMacroNode{
		position: start,
		args:     make(map[string]IEvaluator),
	}

	nameToken := arguments.MatchType(TokenIdentifier)
	if nameToken == nil {
		return nil, arguments.Error("Macro-tag needs at least an identifier as name.", nil)
	}
	macroNode.name = nameToken.Val

	if arguments.MatchOne(TokenSymbol, "(") == nil {
		return nil, arguments.Error("Expected '('.", nil)
	}

	for arguments.Match(TokenSymbol, ")") == nil {
		argNameToken := arguments.MatchType(TokenIdentifier)
		if argNameToken == nil {
			return nil, arguments.Error("Expected argument name as identifier.", nil)
		}
		macroNode.argsOrder = append(macroNode.argsOrder, argNameToken.Val)

		if arguments.Match(TokenSymbol, "=") != nil {
			// Default expression follows
			argDefaultExpr, err := arguments.ParseExpression()
			if err != nil {
				return nil, err
			}
			macroNode.args[argNameToken.Val] = argDefaultExpr
		} else {
			// No default expression
			macroNode.args[argNameToken.Val] = nil
		}

		if arguments.Match(TokenSymbol, ")") != nil {
			break
		}
		if arguments.Match(TokenSymbol, ",") == nil {
			return nil, arguments.Error("Expected ',' or ')'.", nil)
		}
	}

	if arguments.Match(TokenKeyword, "export") != nil {
		macroNode.exported = true
	}

	if arguments.Remaining() > 0 {
		return nil, arguments.Error("Malformed macro-tag.", nil)
	}

	// Body wrapping
	wrapper, endargs, err := doc.WrapUntilTag("endmacro")
	if err != nil {
		return nil, err
	}
	macroNode.wrapper = wrapper

	if endargs.Count() > 0 {
		return nil, endargs.Error("Arguments not allowed here.", nil)
	}

	if macroNode.exported {
		// Now register the macro if it wants to be exported
		_, has := doc.template.exportedMacros[macroNode.name]
		if has {
			return nil, doc.Error(fmt.Sprintf("another macro with name '%s' already exported", macroNode.name), start)
		}
		doc.template.exportedMacros[macroNode.name] = macroNode
	}

	return macroNode, nil
}

func init() {
	RegisterTag("macro", tagMacroParser)
}
