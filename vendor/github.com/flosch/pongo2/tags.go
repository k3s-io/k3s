package pongo2

/* Incomplete:
   -----------

   verbatim (only the "name" argument is missing for verbatim)

   Reconsideration:
   ----------------

   debug (reason: not sure what to output yet)
   regroup / Grouping on other properties (reason: maybe too python-specific; not sure how useful this would be in Go)

   Following built-in tags wont be added:
   --------------------------------------

   csrf_token (reason: web-framework specific)
   load (reason: python-specific)
   url (reason: web-framework specific)
*/

import (
	"fmt"

	"github.com/juju/errors"
)

type INodeTag interface {
	INode
}

// This is the function signature of the tag's parser you will have
// to implement in order to create a new tag.
//
// 'doc' is providing access to the whole document while 'arguments'
// is providing access to the user's arguments to the tag:
//
//     {% your_tag_name some "arguments" 123 %}
//
// start_token will be the *Token with the tag's name in it (here: your_tag_name).
//
// Please see the Parser documentation on how to use the parser.
// See RegisterTag()'s documentation for more information about
// writing a tag as well.
type TagParser func(doc *Parser, start *Token, arguments *Parser) (INodeTag, *Error)

type tag struct {
	name   string
	parser TagParser
}

var tags map[string]*tag

func init() {
	tags = make(map[string]*tag)
}

// Registers a new tag. You usually want to call this
// function in the tag's init() function:
// http://golang.org/doc/effective_go.html#init
//
// See http://www.florian-schlachter.de/post/pongo2/ for more about
// writing filters and tags.
func RegisterTag(name string, parserFn TagParser) error {
	_, existing := tags[name]
	if existing {
		return errors.Errorf("tag with name '%s' is already registered", name)
	}
	tags[name] = &tag{
		name:   name,
		parser: parserFn,
	}
	return nil
}

// Replaces an already registered tag with a new implementation. Use this
// function with caution since it allows you to change existing tag behaviour.
func ReplaceTag(name string, parserFn TagParser) error {
	_, existing := tags[name]
	if !existing {
		return errors.Errorf("tag with name '%s' does not exist (therefore cannot be overridden)", name)
	}
	tags[name] = &tag{
		name:   name,
		parser: parserFn,
	}
	return nil
}

// Tag = "{%" IDENT ARGS "%}"
func (p *Parser) parseTagElement() (INodeTag, *Error) {
	p.Consume() // consume "{%"
	tokenName := p.MatchType(TokenIdentifier)

	// Check for identifier
	if tokenName == nil {
		return nil, p.Error("Tag name must be an identifier.", nil)
	}

	// Check for the existing tag
	tag, exists := tags[tokenName.Val]
	if !exists {
		// Does not exists
		return nil, p.Error(fmt.Sprintf("Tag '%s' not found (or beginning tag not provided)", tokenName.Val), tokenName)
	}

	// Check sandbox tag restriction
	if _, isBanned := p.template.set.bannedTags[tokenName.Val]; isBanned {
		return nil, p.Error(fmt.Sprintf("Usage of tag '%s' is not allowed (sandbox restriction active).", tokenName.Val), tokenName)
	}

	var argsToken []*Token
	for p.Peek(TokenSymbol, "%}") == nil && p.Remaining() > 0 {
		// Add token to args
		argsToken = append(argsToken, p.Current())
		p.Consume() // next token
	}

	// EOF?
	if p.Remaining() == 0 {
		return nil, p.Error("Unexpectedly reached EOF, no tag end found.", p.lastToken)
	}

	p.Match(TokenSymbol, "%}")

	argParser := newParser(p.name, argsToken, p.template)
	if len(argsToken) == 0 {
		// This is done to have nice EOF error messages
		argParser.lastToken = tokenName
	}

	p.template.level++
	defer func() { p.template.level-- }()
	return tag.parser(p, tokenName, argParser)
}
