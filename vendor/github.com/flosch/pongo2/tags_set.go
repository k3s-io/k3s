package pongo2

type tagSetNode struct {
	name       string
	expression IEvaluator
}

func (node *tagSetNode) Execute(ctx *ExecutionContext, writer TemplateWriter) *Error {
	// Evaluate expression
	value, err := node.expression.Evaluate(ctx)
	if err != nil {
		return err
	}

	ctx.Private[node.name] = value
	return nil
}

func tagSetParser(doc *Parser, start *Token, arguments *Parser) (INodeTag, *Error) {
	node := &tagSetNode{}

	// Parse variable name
	typeToken := arguments.MatchType(TokenIdentifier)
	if typeToken == nil {
		return nil, arguments.Error("Expected an identifier.", nil)
	}
	node.name = typeToken.Val

	if arguments.Match(TokenSymbol, "=") == nil {
		return nil, arguments.Error("Expected '='.", nil)
	}

	// Variable expression
	keyExpression, err := arguments.ParseExpression()
	if err != nil {
		return nil, err
	}
	node.expression = keyExpression

	// Remaining arguments
	if arguments.Remaining() > 0 {
		return nil, arguments.Error("Malformed 'set'-tag arguments.", nil)
	}

	return node, nil
}

func init() {
	RegisterTag("set", tagSetParser)
}
