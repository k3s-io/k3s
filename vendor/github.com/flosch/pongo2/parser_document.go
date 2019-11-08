package pongo2

// Doc = { ( Filter | Tag | HTML ) }
func (p *Parser) parseDocElement() (INode, *Error) {
	t := p.Current()

	switch t.Typ {
	case TokenHTML:
		n := &nodeHTML{token: t}
		left := p.PeekTypeN(-1, TokenSymbol)
		right := p.PeekTypeN(1, TokenSymbol)
		n.trimLeft = left != nil && left.TrimWhitespaces
		n.trimRight = right != nil && right.TrimWhitespaces
		p.Consume() // consume HTML element
		return n, nil
	case TokenSymbol:
		switch t.Val {
		case "{{":
			// parse variable
			variable, err := p.parseVariableElement()
			if err != nil {
				return nil, err
			}
			return variable, nil
		case "{%":
			// parse tag
			tag, err := p.parseTagElement()
			if err != nil {
				return nil, err
			}
			return tag, nil
		}
	}
	return nil, p.Error("Unexpected token (only HTML/tags/filters in templates allowed)", t)
}

func (tpl *Template) parse() *Error {
	tpl.parser = newParser(tpl.name, tpl.tokens, tpl)
	doc, err := tpl.parser.parseDocument()
	if err != nil {
		return err
	}
	tpl.root = doc
	return nil
}

func (p *Parser) parseDocument() (*nodeDocument, *Error) {
	doc := &nodeDocument{}

	for p.Remaining() > 0 {
		node, err := p.parseDocElement()
		if err != nil {
			return nil, err
		}
		doc.Nodes = append(doc.Nodes, node)
	}

	return doc, nil
}
