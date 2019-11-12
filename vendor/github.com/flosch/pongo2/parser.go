package pongo2

import (
	"fmt"
	"strings"

	"github.com/juju/errors"
)

type INode interface {
	Execute(*ExecutionContext, TemplateWriter) *Error
}

type IEvaluator interface {
	INode
	GetPositionToken() *Token
	Evaluate(*ExecutionContext) (*Value, *Error)
	FilterApplied(name string) bool
}

// The parser provides you a comprehensive and easy tool to
// work with the template document and arguments provided by
// the user for your custom tag.
//
// The parser works on a token list which will be provided by pongo2.
// A token is a unit you can work with. Tokens are either of type identifier,
// string, number, keyword, HTML or symbol.
//
// (See Token's documentation for more about tokens)
type Parser struct {
	name      string
	idx       int
	tokens    []*Token
	lastToken *Token

	// if the parser parses a template document, here will be
	// a reference to it (needed to access the template through Tags)
	template *Template
}

// Creates a new parser to parse tokens.
// Used inside pongo2 to parse documents and to provide an easy-to-use
// parser for tag authors
func newParser(name string, tokens []*Token, template *Template) *Parser {
	p := &Parser{
		name:     name,
		tokens:   tokens,
		template: template,
	}
	if len(tokens) > 0 {
		p.lastToken = tokens[len(tokens)-1]
	}
	return p
}

// Consume one token. It will be gone forever.
func (p *Parser) Consume() {
	p.ConsumeN(1)
}

// Consume N tokens. They will be gone forever.
func (p *Parser) ConsumeN(count int) {
	p.idx += count
}

// Returns the current token.
func (p *Parser) Current() *Token {
	return p.Get(p.idx)
}

// Returns the CURRENT token if the given type matches.
// Consumes this token on success.
func (p *Parser) MatchType(typ TokenType) *Token {
	if t := p.PeekType(typ); t != nil {
		p.Consume()
		return t
	}
	return nil
}

// Returns the CURRENT token if the given type AND value matches.
// Consumes this token on success.
func (p *Parser) Match(typ TokenType, val string) *Token {
	if t := p.Peek(typ, val); t != nil {
		p.Consume()
		return t
	}
	return nil
}

// Returns the CURRENT token if the given type AND *one* of
// the given values matches.
// Consumes this token on success.
func (p *Parser) MatchOne(typ TokenType, vals ...string) *Token {
	for _, val := range vals {
		if t := p.Peek(typ, val); t != nil {
			p.Consume()
			return t
		}
	}
	return nil
}

// Returns the CURRENT token if the given type matches.
// It DOES NOT consume the token.
func (p *Parser) PeekType(typ TokenType) *Token {
	return p.PeekTypeN(0, typ)
}

// Returns the CURRENT token if the given type AND value matches.
// It DOES NOT consume the token.
func (p *Parser) Peek(typ TokenType, val string) *Token {
	return p.PeekN(0, typ, val)
}

// Returns the CURRENT token if the given type AND *one* of
// the given values matches.
// It DOES NOT consume the token.
func (p *Parser) PeekOne(typ TokenType, vals ...string) *Token {
	for _, v := range vals {
		t := p.PeekN(0, typ, v)
		if t != nil {
			return t
		}
	}
	return nil
}

// Returns the tokens[current position + shift] token if the
// given type AND value matches for that token.
// DOES NOT consume the token.
func (p *Parser) PeekN(shift int, typ TokenType, val string) *Token {
	t := p.Get(p.idx + shift)
	if t != nil {
		if t.Typ == typ && t.Val == val {
			return t
		}
	}
	return nil
}

// Returns the tokens[current position + shift] token if the given type matches.
// DOES NOT consume the token for that token.
func (p *Parser) PeekTypeN(shift int, typ TokenType) *Token {
	t := p.Get(p.idx + shift)
	if t != nil {
		if t.Typ == typ {
			return t
		}
	}
	return nil
}

// Returns the UNCONSUMED token count.
func (p *Parser) Remaining() int {
	return len(p.tokens) - p.idx
}

// Returns the total token count.
func (p *Parser) Count() int {
	return len(p.tokens)
}

// Returns tokens[i] or NIL (if i >= len(tokens))
func (p *Parser) Get(i int) *Token {
	if i < len(p.tokens) && i >= 0 {
		return p.tokens[i]
	}
	return nil
}

// Returns tokens[current-position + shift] or NIL
// (if (current-position + i) >= len(tokens))
func (p *Parser) GetR(shift int) *Token {
	i := p.idx + shift
	return p.Get(i)
}

// Error produces a nice error message and returns an error-object.
// The 'token'-argument is optional. If provided, it will take
// the token's position information. If not provided, it will
// automatically use the CURRENT token's position information.
func (p *Parser) Error(msg string, token *Token) *Error {
	if token == nil {
		// Set current token
		token = p.Current()
		if token == nil {
			// Set to last token
			if len(p.tokens) > 0 {
				token = p.tokens[len(p.tokens)-1]
			}
		}
	}
	var line, col int
	if token != nil {
		line = token.Line
		col = token.Col
	}
	return &Error{
		Template:  p.template,
		Filename:  p.name,
		Sender:    "parser",
		Line:      line,
		Column:    col,
		Token:     token,
		OrigError: errors.New(msg),
	}
}

// Wraps all nodes between starting tag and "{% endtag %}" and provides
// one simple interface to execute the wrapped nodes.
// It returns a parser to process provided arguments to the tag.
func (p *Parser) WrapUntilTag(names ...string) (*NodeWrapper, *Parser, *Error) {
	wrapper := &NodeWrapper{}

	var tagArgs []*Token

	for p.Remaining() > 0 {
		// New tag, check whether we have to stop wrapping here
		if p.Peek(TokenSymbol, "{%") != nil {
			tagIdent := p.PeekTypeN(1, TokenIdentifier)

			if tagIdent != nil {
				// We've found a (!) end-tag

				found := false
				for _, n := range names {
					if tagIdent.Val == n {
						found = true
						break
					}
				}

				// We only process the tag if we've found an end tag
				if found {
					// Okay, endtag found.
					p.ConsumeN(2) // '{%' tagname

					for {
						if p.Match(TokenSymbol, "%}") != nil {
							// Okay, end the wrapping here
							wrapper.Endtag = tagIdent.Val
							return wrapper, newParser(p.template.name, tagArgs, p.template), nil
						}
						t := p.Current()
						p.Consume()
						if t == nil {
							return nil, nil, p.Error("Unexpected EOF.", p.lastToken)
						}
						tagArgs = append(tagArgs, t)
					}
				}
			}

		}

		// Otherwise process next element to be wrapped
		node, err := p.parseDocElement()
		if err != nil {
			return nil, nil, err
		}
		wrapper.nodes = append(wrapper.nodes, node)
	}

	return nil, nil, p.Error(fmt.Sprintf("Unexpected EOF, expected tag %s.", strings.Join(names, " or ")),
		p.lastToken)
}

// Skips all nodes between starting tag and "{% endtag %}"
func (p *Parser) SkipUntilTag(names ...string) *Error {
	for p.Remaining() > 0 {
		// New tag, check whether we have to stop wrapping here
		if p.Peek(TokenSymbol, "{%") != nil {
			tagIdent := p.PeekTypeN(1, TokenIdentifier)

			if tagIdent != nil {
				// We've found a (!) end-tag

				found := false
				for _, n := range names {
					if tagIdent.Val == n {
						found = true
						break
					}
				}

				// We only process the tag if we've found an end tag
				if found {
					// Okay, endtag found.
					p.ConsumeN(2) // '{%' tagname

					for {
						if p.Match(TokenSymbol, "%}") != nil {
							// Done skipping, exit.
							return nil
						}
					}
				}
			}
		}
		t := p.Current()
		p.Consume()
		if t == nil {
			return p.Error("Unexpected EOF.", p.lastToken)
		}
	}

	return p.Error(fmt.Sprintf("Unexpected EOF, expected tag %s.", strings.Join(names, " or ")), p.lastToken)
}
