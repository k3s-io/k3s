package pongo2

type tagFirstofNode struct {
	position *Token
	args     []IEvaluator
}

func (node *tagFirstofNode) Execute(ctx *ExecutionContext, writer TemplateWriter) *Error {
	for _, arg := range node.args {
		val, err := arg.Evaluate(ctx)
		if err != nil {
			return err
		}

		if val.IsTrue() {
			if ctx.Autoescape && !arg.FilterApplied("safe") {
				val, err = ApplyFilter("escape", val, nil)
				if err != nil {
					return err
				}
			}

			writer.WriteString(val.String())
			return nil
		}
	}

	return nil
}

func tagFirstofParser(doc *Parser, start *Token, arguments *Parser) (INodeTag, *Error) {
	firstofNode := &tagFirstofNode{
		position: start,
	}

	for arguments.Remaining() > 0 {
		node, err := arguments.ParseExpression()
		if err != nil {
			return nil, err
		}
		firstofNode.args = append(firstofNode.args, node)
	}

	return firstofNode, nil
}

func init() {
	RegisterTag("firstof", tagFirstofParser)
}
