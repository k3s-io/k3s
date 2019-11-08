package pongo2

type NodeWrapper struct {
	Endtag string
	nodes  []INode
}

func (wrapper *NodeWrapper) Execute(ctx *ExecutionContext, writer TemplateWriter) *Error {
	for _, n := range wrapper.nodes {
		err := n.Execute(ctx, writer)
		if err != nil {
			return err
		}
	}
	return nil
}
