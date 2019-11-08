package pongo2

import (
	"fmt"
	"math"
)

type tagWidthratioNode struct {
	position     *Token
	current, max IEvaluator
	width        IEvaluator
	ctxName      string
}

func (node *tagWidthratioNode) Execute(ctx *ExecutionContext, writer TemplateWriter) *Error {
	current, err := node.current.Evaluate(ctx)
	if err != nil {
		return err
	}

	max, err := node.max.Evaluate(ctx)
	if err != nil {
		return err
	}

	width, err := node.width.Evaluate(ctx)
	if err != nil {
		return err
	}

	value := int(math.Ceil(current.Float()/max.Float()*width.Float() + 0.5))

	if node.ctxName == "" {
		writer.WriteString(fmt.Sprintf("%d", value))
	} else {
		ctx.Private[node.ctxName] = value
	}

	return nil
}

func tagWidthratioParser(doc *Parser, start *Token, arguments *Parser) (INodeTag, *Error) {
	widthratioNode := &tagWidthratioNode{
		position: start,
	}

	current, err := arguments.ParseExpression()
	if err != nil {
		return nil, err
	}
	widthratioNode.current = current

	max, err := arguments.ParseExpression()
	if err != nil {
		return nil, err
	}
	widthratioNode.max = max

	width, err := arguments.ParseExpression()
	if err != nil {
		return nil, err
	}
	widthratioNode.width = width

	if arguments.MatchOne(TokenKeyword, "as") != nil {
		// Name follows
		nameToken := arguments.MatchType(TokenIdentifier)
		if nameToken == nil {
			return nil, arguments.Error("Expected name (identifier).", nil)
		}
		widthratioNode.ctxName = nameToken.Val
	}

	if arguments.Remaining() > 0 {
		return nil, arguments.Error("Malformed widthratio-tag arguments.", nil)
	}

	return widthratioNode, nil
}

func init() {
	RegisterTag("widthratio", tagWidthratioParser)
}
