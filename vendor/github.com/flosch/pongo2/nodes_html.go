package pongo2

import (
	"strings"
)

type nodeHTML struct {
	token *Token
	trimLeft bool
	trimRight bool
}

func (n *nodeHTML) Execute(ctx *ExecutionContext, writer TemplateWriter) *Error {
	res := n.token.Val
	if n.trimLeft {
		res = strings.TrimLeft(res, tokenSpaceChars)
	}
	if n.trimRight {
		res = strings.TrimRight(res, tokenSpaceChars)
	}
	writer.WriteString(res)
	return nil
}
