package pongo2

import (
	"io/ioutil"
)

type tagSSINode struct {
	filename string
	content  string
	template *Template
}

func (node *tagSSINode) Execute(ctx *ExecutionContext, writer TemplateWriter) *Error {
	if node.template != nil {
		// Execute the template within the current context
		includeCtx := make(Context)
		includeCtx.Update(ctx.Public)
		includeCtx.Update(ctx.Private)

		err := node.template.execute(includeCtx, writer)
		if err != nil {
			return err.(*Error)
		}
	} else {
		// Just print out the content
		writer.WriteString(node.content)
	}
	return nil
}

func tagSSIParser(doc *Parser, start *Token, arguments *Parser) (INodeTag, *Error) {
	SSINode := &tagSSINode{}

	if fileToken := arguments.MatchType(TokenString); fileToken != nil {
		SSINode.filename = fileToken.Val

		if arguments.Match(TokenIdentifier, "parsed") != nil {
			// parsed
			temporaryTpl, err := doc.template.set.FromFile(doc.template.set.resolveFilename(doc.template, fileToken.Val))
			if err != nil {
				return nil, err.(*Error).updateFromTokenIfNeeded(doc.template, fileToken)
			}
			SSINode.template = temporaryTpl
		} else {
			// plaintext
			buf, err := ioutil.ReadFile(doc.template.set.resolveFilename(doc.template, fileToken.Val))
			if err != nil {
				return nil, (&Error{
					Sender:    "tag:ssi",
					OrigError: err,
				}).updateFromTokenIfNeeded(doc.template, fileToken)
			}
			SSINode.content = string(buf)
		}
	} else {
		return nil, arguments.Error("First argument must be a string.", nil)
	}

	if arguments.Remaining() > 0 {
		return nil, arguments.Error("Malformed SSI-tag argument.", nil)
	}

	return SSINode, nil
}

func init() {
	RegisterTag("ssi", tagSSIParser)
}
