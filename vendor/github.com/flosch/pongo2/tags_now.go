package pongo2

import (
	"time"
)

type tagNowNode struct {
	position *Token
	format   string
	fake     bool
}

func (node *tagNowNode) Execute(ctx *ExecutionContext, writer TemplateWriter) *Error {
	var t time.Time
	if node.fake {
		t = time.Date(2014, time.February, 05, 18, 31, 45, 00, time.UTC)
	} else {
		t = time.Now()
	}

	writer.WriteString(t.Format(node.format))

	return nil
}

func tagNowParser(doc *Parser, start *Token, arguments *Parser) (INodeTag, *Error) {
	nowNode := &tagNowNode{
		position: start,
	}

	formatToken := arguments.MatchType(TokenString)
	if formatToken == nil {
		return nil, arguments.Error("Expected a format string.", nil)
	}
	nowNode.format = formatToken.Val

	if arguments.MatchOne(TokenIdentifier, "fake") != nil {
		nowNode.fake = true
	}

	if arguments.Remaining() > 0 {
		return nil, arguments.Error("Malformed now-tag arguments.", nil)
	}

	return nowNode, nil
}

func init() {
	RegisterTag("now", tagNowParser)
}
