package pongo2

// The root document
type nodeDocument struct {
	Nodes []INode
}

func (doc *nodeDocument) Execute(ctx *ExecutionContext, writer TemplateWriter) *Error {
	for _, n := range doc.Nodes {
		err := n.Execute(ctx, writer)
		if err != nil {
			return err
		}
	}
	return nil
}
