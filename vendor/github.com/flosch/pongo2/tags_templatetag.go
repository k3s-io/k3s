package pongo2

type tagTemplateTagNode struct {
	content string
}

var templateTagMapping = map[string]string{
	"openblock":     "{%",
	"closeblock":    "%}",
	"openvariable":  "{{",
	"closevariable": "}}",
	"openbrace":     "{",
	"closebrace":    "}",
	"opencomment":   "{#",
	"closecomment":  "#}",
}

func (node *tagTemplateTagNode) Execute(ctx *ExecutionContext, writer TemplateWriter) *Error {
	writer.WriteString(node.content)
	return nil
}

func tagTemplateTagParser(doc *Parser, start *Token, arguments *Parser) (INodeTag, *Error) {
	ttNode := &tagTemplateTagNode{}

	if argToken := arguments.MatchType(TokenIdentifier); argToken != nil {
		output, found := templateTagMapping[argToken.Val]
		if !found {
			return nil, arguments.Error("Argument not found", argToken)
		}
		ttNode.content = output
	} else {
		return nil, arguments.Error("Identifier expected.", nil)
	}

	if arguments.Remaining() > 0 {
		return nil, arguments.Error("Malformed templatetag-tag argument.", nil)
	}

	return ttNode, nil
}

func init() {
	RegisterTag("templatetag", tagTemplateTagParser)
}
