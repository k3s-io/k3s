package pongo2

type tagIfNode struct {
	conditions []IEvaluator
	wrappers   []*NodeWrapper
}

func (node *tagIfNode) Execute(ctx *ExecutionContext, writer TemplateWriter) *Error {
	for i, condition := range node.conditions {
		result, err := condition.Evaluate(ctx)
		if err != nil {
			return err
		}

		if result.IsTrue() {
			return node.wrappers[i].Execute(ctx, writer)
		}
		// Last condition?
		if len(node.conditions) == i+1 && len(node.wrappers) > i+1 {
			return node.wrappers[i+1].Execute(ctx, writer)
		}
	}
	return nil
}

func tagIfParser(doc *Parser, start *Token, arguments *Parser) (INodeTag, *Error) {
	ifNode := &tagIfNode{}

	// Parse first and main IF condition
	condition, err := arguments.ParseExpression()
	if err != nil {
		return nil, err
	}
	ifNode.conditions = append(ifNode.conditions, condition)

	if arguments.Remaining() > 0 {
		return nil, arguments.Error("If-condition is malformed.", nil)
	}

	// Check the rest
	for {
		wrapper, tagArgs, err := doc.WrapUntilTag("elif", "else", "endif")
		if err != nil {
			return nil, err
		}
		ifNode.wrappers = append(ifNode.wrappers, wrapper)

		if wrapper.Endtag == "elif" {
			// elif can take a condition
			condition, err = tagArgs.ParseExpression()
			if err != nil {
				return nil, err
			}
			ifNode.conditions = append(ifNode.conditions, condition)

			if tagArgs.Remaining() > 0 {
				return nil, tagArgs.Error("Elif-condition is malformed.", nil)
			}
		} else {
			if tagArgs.Count() > 0 {
				// else/endif can't take any conditions
				return nil, tagArgs.Error("Arguments not allowed here.", nil)
			}
		}

		if wrapper.Endtag == "endif" {
			break
		}
	}

	return ifNode, nil
}

func init() {
	RegisterTag("if", tagIfParser)
}
