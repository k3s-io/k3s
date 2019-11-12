package pongo2

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/juju/errors"
)

const (
	TokenError = iota
	EOF

	TokenHTML

	TokenKeyword
	TokenIdentifier
	TokenString
	TokenNumber
	TokenSymbol
)

var (
	tokenSpaceChars                = " \n\r\t"
	tokenIdentifierChars           = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ_"
	tokenIdentifierCharsWithDigits = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ_0123456789"
	tokenDigits                    = "0123456789"

	// Available symbols in pongo2 (within filters/tag)
	TokenSymbols = []string{
		// 3-Char symbols
		"{{-", "-}}", "{%-", "-%}",

		// 2-Char symbols
		"==", ">=", "<=", "&&", "||", "{{", "}}", "{%", "%}", "!=", "<>",

		// 1-Char symbol
		"(", ")", "+", "-", "*", "<", ">", "/", "^", ",", ".", "!", "|", ":", "=", "%",
	}

	// Available keywords in pongo2
	TokenKeywords = []string{"in", "and", "or", "not", "true", "false", "as", "export"}
)

type TokenType int
type Token struct {
	Filename        string
	Typ             TokenType
	Val             string
	Line            int
	Col             int
	TrimWhitespaces bool
}

type lexerStateFn func() lexerStateFn
type lexer struct {
	name      string
	input     string
	start     int // start pos of the item
	pos       int // current pos
	width     int // width of last rune
	tokens    []*Token
	errored   bool
	startline int
	startcol  int
	line      int
	col       int

	inVerbatim   bool
	verbatimName string
}

func (t *Token) String() string {
	val := t.Val
	if len(val) > 1000 {
		val = fmt.Sprintf("%s...%s", val[:10], val[len(val)-5:len(val)])
	}

	typ := ""
	switch t.Typ {
	case TokenHTML:
		typ = "HTML"
	case TokenError:
		typ = "Error"
	case TokenIdentifier:
		typ = "Identifier"
	case TokenKeyword:
		typ = "Keyword"
	case TokenNumber:
		typ = "Number"
	case TokenString:
		typ = "String"
	case TokenSymbol:
		typ = "Symbol"
	default:
		typ = "Unknown"
	}

	return fmt.Sprintf("<Token Typ=%s (%d) Val='%s' Line=%d Col=%d, WT=%t>",
		typ, t.Typ, val, t.Line, t.Col, t.TrimWhitespaces)
}

func lex(name string, input string) ([]*Token, *Error) {
	l := &lexer{
		name:      name,
		input:     input,
		tokens:    make([]*Token, 0, 100),
		line:      1,
		col:       1,
		startline: 1,
		startcol:  1,
	}
	l.run()
	if l.errored {
		errtoken := l.tokens[len(l.tokens)-1]
		return nil, &Error{
			Filename:  name,
			Line:      errtoken.Line,
			Column:    errtoken.Col,
			Sender:    "lexer",
			OrigError: errors.New(errtoken.Val),
		}
	}
	return l.tokens, nil
}

func (l *lexer) value() string {
	return l.input[l.start:l.pos]
}

func (l *lexer) length() int {
	return l.pos - l.start
}

func (l *lexer) emit(t TokenType) {
	tok := &Token{
		Filename: l.name,
		Typ:      t,
		Val:      l.value(),
		Line:     l.startline,
		Col:      l.startcol,
	}

	if t == TokenString {
		// Escape sequence \" in strings
		tok.Val = strings.Replace(tok.Val, `\"`, `"`, -1)
		tok.Val = strings.Replace(tok.Val, `\\`, `\`, -1)
	}

	if t == TokenSymbol && len(tok.Val) == 3 && (strings.HasSuffix(tok.Val, "-") || strings.HasPrefix(tok.Val, "-")) {
		tok.TrimWhitespaces = true
		tok.Val = strings.Replace(tok.Val, "-", "", -1)
	}

	l.tokens = append(l.tokens, tok)
	l.start = l.pos
	l.startline = l.line
	l.startcol = l.col
}

func (l *lexer) next() rune {
	if l.pos >= len(l.input) {
		l.width = 0
		return EOF
	}
	r, w := utf8.DecodeRuneInString(l.input[l.pos:])
	l.width = w
	l.pos += l.width
	l.col += l.width
	return r
}

func (l *lexer) backup() {
	l.pos -= l.width
	l.col -= l.width
}

func (l *lexer) peek() rune {
	r := l.next()
	l.backup()
	return r
}

func (l *lexer) ignore() {
	l.start = l.pos
	l.startline = l.line
	l.startcol = l.col
}

func (l *lexer) accept(what string) bool {
	if strings.IndexRune(what, l.next()) >= 0 {
		return true
	}
	l.backup()
	return false
}

func (l *lexer) acceptRun(what string) {
	for strings.IndexRune(what, l.next()) >= 0 {
	}
	l.backup()
}

func (l *lexer) errorf(format string, args ...interface{}) lexerStateFn {
	t := &Token{
		Filename: l.name,
		Typ:      TokenError,
		Val:      fmt.Sprintf(format, args...),
		Line:     l.startline,
		Col:      l.startcol,
	}
	l.tokens = append(l.tokens, t)
	l.errored = true
	l.startline = l.line
	l.startcol = l.col
	return nil
}

func (l *lexer) eof() bool {
	return l.start >= len(l.input)-1
}

func (l *lexer) run() {
	for {
		// TODO: Support verbatim tag names
		// https://docs.djangoproject.com/en/dev/ref/templates/builtins/#verbatim
		if l.inVerbatim {
			name := l.verbatimName
			if name != "" {
				name += " "
			}
			if strings.HasPrefix(l.input[l.pos:], fmt.Sprintf("{%% endverbatim %s%%}", name)) { // end verbatim
				if l.pos > l.start {
					l.emit(TokenHTML)
				}
				w := len("{% endverbatim %}")
				l.pos += w
				l.col += w
				l.ignore()
				l.inVerbatim = false
			}
		} else if strings.HasPrefix(l.input[l.pos:], "{% verbatim %}") { // tag
			if l.pos > l.start {
				l.emit(TokenHTML)
			}
			l.inVerbatim = true
			w := len("{% verbatim %}")
			l.pos += w
			l.col += w
			l.ignore()
		}

		if !l.inVerbatim {
			// Ignore single-line comments {# ... #}
			if strings.HasPrefix(l.input[l.pos:], "{#") {
				if l.pos > l.start {
					l.emit(TokenHTML)
				}

				l.pos += 2 // pass '{#'
				l.col += 2

				for {
					switch l.peek() {
					case EOF:
						l.errorf("Single-line comment not closed.")
						return
					case '\n':
						l.errorf("Newline not permitted in a single-line comment.")
						return
					}

					if strings.HasPrefix(l.input[l.pos:], "#}") {
						l.pos += 2 // pass '#}'
						l.col += 2
						break
					}

					l.next()
				}
				l.ignore() // ignore whole comment

				// Comment skipped
				continue // next token
			}

			if strings.HasPrefix(l.input[l.pos:], "{{") || // variable
				strings.HasPrefix(l.input[l.pos:], "{%") { // tag
				if l.pos > l.start {
					l.emit(TokenHTML)
				}
				l.tokenize()
				if l.errored {
					return
				}
				continue
			}
		}

		switch l.peek() {
		case '\n':
			l.line++
			l.col = 0
		}
		if l.next() == EOF {
			break
		}
	}

	if l.pos > l.start {
		l.emit(TokenHTML)
	}

	if l.inVerbatim {
		l.errorf("verbatim-tag not closed, got EOF.")
	}
}

func (l *lexer) tokenize() {
	for state := l.stateCode; state != nil; {
		state = state()
	}
}

func (l *lexer) stateCode() lexerStateFn {
outer_loop:
	for {
		switch {
		case l.accept(tokenSpaceChars):
			if l.value() == "\n" {
				return l.errorf("Newline not allowed within tag/variable.")
			}
			l.ignore()
			continue
		case l.accept(tokenIdentifierChars):
			return l.stateIdentifier
		case l.accept(tokenDigits):
			return l.stateNumber
		case l.accept(`"'`):
			return l.stateString
		}

		// Check for symbol
		for _, sym := range TokenSymbols {
			if strings.HasPrefix(l.input[l.start:], sym) {
				l.pos += len(sym)
				l.col += l.length()
				l.emit(TokenSymbol)

				if sym == "%}" || sym == "-%}" || sym == "}}" || sym == "-}}" {
					// Tag/variable end, return after emit
					return nil
				}

				continue outer_loop
			}
		}

		break
	}

	// Normal shut down
	return nil
}

func (l *lexer) stateIdentifier() lexerStateFn {
	l.acceptRun(tokenIdentifierChars)
	l.acceptRun(tokenIdentifierCharsWithDigits)
	for _, kw := range TokenKeywords {
		if kw == l.value() {
			l.emit(TokenKeyword)
			return l.stateCode
		}
	}
	l.emit(TokenIdentifier)
	return l.stateCode
}

func (l *lexer) stateNumber() lexerStateFn {
	l.acceptRun(tokenDigits)
	if l.accept(tokenIdentifierCharsWithDigits) {
		// This seems to be an identifier starting with a number.
		// See https://github.com/flosch/pongo2/issues/151
		return l.stateIdentifier()
	}
	/*
		Maybe context-sensitive number lexing?
		* comments.0.Text // first comment
		* usercomments.1.0 // second user, first comment
		* if (score >= 8.5) // 8.5 as a number

		if l.peek() == '.' {
			l.accept(".")
			if !l.accept(tokenDigits) {
				return l.errorf("Malformed number.")
			}
			l.acceptRun(tokenDigits)
		}
	*/
	l.emit(TokenNumber)
	return l.stateCode
}

func (l *lexer) stateString() lexerStateFn {
	quotationMark := l.value()
	l.ignore()
	l.startcol-- // we're starting the position at the first "
	for !l.accept(quotationMark) {
		switch l.next() {
		case '\\':
			// escape sequence
			switch l.peek() {
			case '"', '\\':
				l.next()
			default:
				return l.errorf("Unknown escape sequence: \\%c", l.peek())
			}
		case EOF:
			return l.errorf("Unexpected EOF, string not closed.")
		case '\n':
			return l.errorf("Newline in string is not allowed.")
		}
	}
	l.backup()
	l.emit(TokenString)

	l.next()
	l.ignore()

	return l.stateCode
}
