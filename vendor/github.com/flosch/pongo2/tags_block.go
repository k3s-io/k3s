package pongo2

import (
	"bytes"
	"fmt"
)

type tagBlockNode struct {
	name string
}

func (node *tagBlockNode) getBlockWrappers(tpl *Template) []*NodeWrapper {
	nodeWrappers := make([]*NodeWrapper, 0)
	var t *NodeWrapper

	for tpl != nil {
		t = tpl.blocks[node.name]
		if t != nil {
			nodeWrappers = append(nodeWrappers, t)
		}
		tpl = tpl.child
	}

	return nodeWrappers
}

func (node *tagBlockNode) Execute(ctx *ExecutionContext, writer TemplateWriter) *Error {
	tpl := ctx.template
	if tpl == nil {
		panic("internal error: tpl == nil")
	}

	// Determine the block to execute
	blockWrappers := node.getBlockWrappers(tpl)
	lenBlockWrappers := len(blockWrappers)

	if lenBlockWrappers == 0 {
		return ctx.Error("internal error: len(block_wrappers) == 0 in tagBlockNode.Execute()", nil)
	}

	blockWrapper := blockWrappers[lenBlockWrappers-1]
	ctx.Private["block"] = tagBlockInformation{
		ctx:      ctx,
		wrappers: blockWrappers[0 : lenBlockWrappers-1],
	}
	err := blockWrapper.Execute(ctx, writer)
	if err != nil {
		return err
	}

	return nil
}

type tagBlockInformation struct {
	ctx      *ExecutionContext
	wrappers []*NodeWrapper
}

func (t tagBlockInformation) Super() string {
	lenWrappers := len(t.wrappers)

	if lenWrappers == 0 {
		return ""
	}

	superCtx := NewChildExecutionContext(t.ctx)
	superCtx.Private["block"] = tagBlockInformation{
		ctx:      t.ctx,
		wrappers: t.wrappers[0 : lenWrappers-1],
	}

	blockWrapper := t.wrappers[lenWrappers-1]
	buf := bytes.NewBufferString("")
	err := blockWrapper.Execute(superCtx, &templateWriter{buf})
	if err != nil {
		return ""
	}
	return buf.String()
}

func tagBlockParser(doc *Parser, start *Token, arguments *Parser) (INodeTag, *Error) {
	if arguments.Count() == 0 {
		return nil, arguments.Error("Tag 'block' requires an identifier.", nil)
	}

	nameToken := arguments.MatchType(TokenIdentifier)
	if nameToken == nil {
		return nil, arguments.Error("First argument for tag 'block' must be an identifier.", nil)
	}

	if arguments.Remaining() != 0 {
		return nil, arguments.Error("Tag 'block' takes exactly 1 argument (an identifier).", nil)
	}

	wrapper, endtagargs, err := doc.WrapUntilTag("endblock")
	if err != nil {
		return nil, err
	}
	if endtagargs.Remaining() > 0 {
		endtagnameToken := endtagargs.MatchType(TokenIdentifier)
		if endtagnameToken != nil {
			if endtagnameToken.Val != nameToken.Val {
				return nil, endtagargs.Error(fmt.Sprintf("Name for 'endblock' must equal to 'block'-tag's name ('%s' != '%s').",
					nameToken.Val, endtagnameToken.Val), nil)
			}
		}

		if endtagnameToken == nil || endtagargs.Remaining() > 0 {
			return nil, endtagargs.Error("Either no or only one argument (identifier) allowed for 'endblock'.", nil)
		}
	}

	tpl := doc.template
	if tpl == nil {
		panic("internal error: tpl == nil")
	}
	_, hasBlock := tpl.blocks[nameToken.Val]
	if !hasBlock {
		tpl.blocks[nameToken.Val] = wrapper
	} else {
		return nil, arguments.Error(fmt.Sprintf("Block named '%s' already defined", nameToken.Val), nil)
	}

	return &tagBlockNode{name: nameToken.Val}, nil
}

func init() {
	RegisterTag("block", tagBlockParser)
}
