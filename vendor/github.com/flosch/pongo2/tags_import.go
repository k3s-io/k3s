package pongo2

import (
	"fmt"
)

type tagImportNode struct {
	position *Token
	filename string
	macros   map[string]*tagMacroNode // alias/name -> macro instance
}

func (node *tagImportNode) Execute(ctx *ExecutionContext, writer TemplateWriter) *Error {
	for name, macro := range node.macros {
		func(name string, macro *tagMacroNode) {
			ctx.Private[name] = func(args ...*Value) *Value {
				return macro.call(ctx, args...)
			}
		}(name, macro)
	}
	return nil
}

func tagImportParser(doc *Parser, start *Token, arguments *Parser) (INodeTag, *Error) {
	importNode := &tagImportNode{
		position: start,
		macros:   make(map[string]*tagMacroNode),
	}

	filenameToken := arguments.MatchType(TokenString)
	if filenameToken == nil {
		return nil, arguments.Error("Import-tag needs a filename as string.", nil)
	}

	importNode.filename = doc.template.set.resolveFilename(doc.template, filenameToken.Val)

	if arguments.Remaining() == 0 {
		return nil, arguments.Error("You must at least specify one macro to import.", nil)
	}

	// Compile the given template
	tpl, err := doc.template.set.FromFile(importNode.filename)
	if err != nil {
		return nil, err.(*Error).updateFromTokenIfNeeded(doc.template, start)
	}

	for arguments.Remaining() > 0 {
		macroNameToken := arguments.MatchType(TokenIdentifier)
		if macroNameToken == nil {
			return nil, arguments.Error("Expected macro name (identifier).", nil)
		}

		asName := macroNameToken.Val
		if arguments.Match(TokenKeyword, "as") != nil {
			aliasToken := arguments.MatchType(TokenIdentifier)
			if aliasToken == nil {
				return nil, arguments.Error("Expected macro alias name (identifier).", nil)
			}
			asName = aliasToken.Val
		}

		macroInstance, has := tpl.exportedMacros[macroNameToken.Val]
		if !has {
			return nil, arguments.Error(fmt.Sprintf("Macro '%s' not found (or not exported) in '%s'.", macroNameToken.Val,
				importNode.filename), macroNameToken)
		}

		importNode.macros[asName] = macroInstance

		if arguments.Remaining() == 0 {
			break
		}

		if arguments.Match(TokenSymbol, ",") == nil {
			return nil, arguments.Error("Expected ','.", nil)
		}
	}

	return importNode, nil
}

func init() {
	RegisterTag("import", tagImportParser)
}
