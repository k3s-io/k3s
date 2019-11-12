package pongo2

type tagExtendsNode struct {
	filename string
}

func (node *tagExtendsNode) Execute(ctx *ExecutionContext, writer TemplateWriter) *Error {
	return nil
}

func tagExtendsParser(doc *Parser, start *Token, arguments *Parser) (INodeTag, *Error) {
	extendsNode := &tagExtendsNode{}

	if doc.template.level > 1 {
		return nil, arguments.Error("The 'extends' tag can only defined on root level.", start)
	}

	if doc.template.parent != nil {
		// Already one parent
		return nil, arguments.Error("This template has already one parent.", start)
	}

	if filenameToken := arguments.MatchType(TokenString); filenameToken != nil {
		// prepared, static template

		// Get parent's filename
		parentFilename := doc.template.set.resolveFilename(doc.template, filenameToken.Val)

		// Parse the parent
		parentTemplate, err := doc.template.set.FromFile(parentFilename)
		if err != nil {
			return nil, err.(*Error)
		}

		// Keep track of things
		parentTemplate.child = doc.template
		doc.template.parent = parentTemplate
		extendsNode.filename = parentFilename
	} else {
		return nil, arguments.Error("Tag 'extends' requires a template filename as string.", nil)
	}

	if arguments.Remaining() > 0 {
		return nil, arguments.Error("Tag 'extends' does only take 1 argument.", nil)
	}

	return extendsNode, nil
}

func init() {
	RegisterTag("extends", tagExtendsParser)
}
