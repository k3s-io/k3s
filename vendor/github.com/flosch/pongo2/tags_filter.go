package pongo2

import (
	"bytes"
)

type nodeFilterCall struct {
	name      string
	paramExpr IEvaluator
}

type tagFilterNode struct {
	position    *Token
	bodyWrapper *NodeWrapper
	filterChain []*nodeFilterCall
}

func (node *tagFilterNode) Execute(ctx *ExecutionContext, writer TemplateWriter) *Error {
	temp := bytes.NewBuffer(make([]byte, 0, 1024)) // 1 KiB size

	err := node.bodyWrapper.Execute(ctx, temp)
	if err != nil {
		return err
	}

	value := AsValue(temp.String())

	for _, call := range node.filterChain {
		var param *Value
		if call.paramExpr != nil {
			param, err = call.paramExpr.Evaluate(ctx)
			if err != nil {
				return err
			}
		} else {
			param = AsValue(nil)
		}
		value, err = ApplyFilter(call.name, value, param)
		if err != nil {
			return ctx.Error(err.Error(), node.position)
		}
	}

	writer.WriteString(value.String())

	return nil
}

func tagFilterParser(doc *Parser, start *Token, arguments *Parser) (INodeTag, *Error) {
	filterNode := &tagFilterNode{
		position: start,
	}

	wrapper, _, err := doc.WrapUntilTag("endfilter")
	if err != nil {
		return nil, err
	}
	filterNode.bodyWrapper = wrapper

	for arguments.Remaining() > 0 {
		filterCall := &nodeFilterCall{}

		nameToken := arguments.MatchType(TokenIdentifier)
		if nameToken == nil {
			return nil, arguments.Error("Expected a filter name (identifier).", nil)
		}
		filterCall.name = nameToken.Val

		if arguments.MatchOne(TokenSymbol, ":") != nil {
			// Filter parameter
			// NOTICE: we can't use ParseExpression() here, because it would parse the next filter "|..." as well in the argument list
			expr, err := arguments.parseVariableOrLiteral()
			if err != nil {
				return nil, err
			}
			filterCall.paramExpr = expr
		}

		filterNode.filterChain = append(filterNode.filterChain, filterCall)

		if arguments.MatchOne(TokenSymbol, "|") == nil {
			break
		}
	}

	if arguments.Remaining() > 0 {
		return nil, arguments.Error("Malformed filter-tag arguments.", nil)
	}

	return filterNode, nil
}

func init() {
	RegisterTag("filter", tagFilterParser)
}
