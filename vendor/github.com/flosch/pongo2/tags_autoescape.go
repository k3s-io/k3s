package pongo2

type tagAutoescapeNode struct {
	wrapper    *NodeWrapper
	autoescape bool
}

func (node *tagAutoescapeNode) Execute(ctx *ExecutionContext, writer TemplateWriter) *Error {
	old := ctx.Autoescape
	ctx.Autoescape = node.autoescape

	err := node.wrapper.Execute(ctx, writer)
	if err != nil {
		return err
	}

	ctx.Autoescape = old

	return nil
}

func tagAutoescapeParser(doc *Parser, start *Token, arguments *Parser) (INodeTag, *Error) {
	autoescapeNode := &tagAutoescapeNode{}

	wrapper, _, err := doc.WrapUntilTag("endautoescape")
	if err != nil {
		return nil, err
	}
	autoescapeNode.wrapper = wrapper

	modeToken := arguments.MatchType(TokenIdentifier)
	if modeToken == nil {
		return nil, arguments.Error("A mode is required for autoescape-tag.", nil)
	}
	if modeToken.Val == "on" {
		autoescapeNode.autoescape = true
	} else if modeToken.Val == "off" {
		autoescapeNode.autoescape = false
	} else {
		return nil, arguments.Error("Only 'on' or 'off' is valid as an autoescape-mode.", nil)
	}

	if arguments.Remaining() > 0 {
		return nil, arguments.Error("Malformed autoescape-tag arguments.", nil)
	}

	return autoescapeNode, nil
}

func init() {
	RegisterTag("autoescape", tagAutoescapeParser)
}
