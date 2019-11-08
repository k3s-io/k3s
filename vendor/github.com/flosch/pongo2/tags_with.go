package pongo2

type tagWithNode struct {
	withPairs map[string]IEvaluator
	wrapper   *NodeWrapper
}

func (node *tagWithNode) Execute(ctx *ExecutionContext, writer TemplateWriter) *Error {
	//new context for block
	withctx := NewChildExecutionContext(ctx)

	// Put all custom with-pairs into the context
	for key, value := range node.withPairs {
		val, err := value.Evaluate(ctx)
		if err != nil {
			return err
		}
		withctx.Private[key] = val
	}

	return node.wrapper.Execute(withctx, writer)
}

func tagWithParser(doc *Parser, start *Token, arguments *Parser) (INodeTag, *Error) {
	withNode := &tagWithNode{
		withPairs: make(map[string]IEvaluator),
	}

	if arguments.Count() == 0 {
		return nil, arguments.Error("Tag 'with' requires at least one argument.", nil)
	}

	wrapper, endargs, err := doc.WrapUntilTag("endwith")
	if err != nil {
		return nil, err
	}
	withNode.wrapper = wrapper

	if endargs.Count() > 0 {
		return nil, endargs.Error("Arguments not allowed here.", nil)
	}

	// Scan through all arguments to see which style the user uses (old or new style).
	// If we find any "as" keyword we will enforce old style; otherwise we will use new style.
	oldStyle := false // by default we're using the new_style
	for i := 0; i < arguments.Count(); i++ {
		if arguments.PeekN(i, TokenKeyword, "as") != nil {
			oldStyle = true
			break
		}
	}

	for arguments.Remaining() > 0 {
		if oldStyle {
			valueExpr, err := arguments.ParseExpression()
			if err != nil {
				return nil, err
			}
			if arguments.Match(TokenKeyword, "as") == nil {
				return nil, arguments.Error("Expected 'as' keyword.", nil)
			}
			keyToken := arguments.MatchType(TokenIdentifier)
			if keyToken == nil {
				return nil, arguments.Error("Expected an identifier", nil)
			}
			withNode.withPairs[keyToken.Val] = valueExpr
		} else {
			keyToken := arguments.MatchType(TokenIdentifier)
			if keyToken == nil {
				return nil, arguments.Error("Expected an identifier", nil)
			}
			if arguments.Match(TokenSymbol, "=") == nil {
				return nil, arguments.Error("Expected '='.", nil)
			}
			valueExpr, err := arguments.ParseExpression()
			if err != nil {
				return nil, err
			}
			withNode.withPairs[keyToken.Val] = valueExpr
		}
	}

	return withNode, nil
}

func init() {
	RegisterTag("with", tagWithParser)
}
