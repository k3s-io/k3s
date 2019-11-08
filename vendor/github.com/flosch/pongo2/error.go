package pongo2

import (
	"bufio"
	"fmt"
	"os"
)

// The Error type is being used to address an error during lexing, parsing or
// execution. If you want to return an error object (for example in your own
// tag or filter) fill this object with as much information as you have.
// Make sure "Sender" is always given (if you're returning an error within
// a filter, make Sender equals 'filter:yourfilter'; same goes for tags: 'tag:mytag').
// It's okay if you only fill in ErrorMsg if you don't have any other details at hand.
type Error struct {
	Template  *Template
	Filename  string
	Line      int
	Column    int
	Token     *Token
	Sender    string
	OrigError error
}

func (e *Error) updateFromTokenIfNeeded(template *Template, t *Token) *Error {
	if e.Template == nil {
		e.Template = template
	}

	if e.Token == nil {
		e.Token = t
		if e.Line <= 0 {
			e.Line = t.Line
			e.Column = t.Col
		}
	}

	return e
}

// Returns a nice formatted error string.
func (e *Error) Error() string {
	s := "[Error"
	if e.Sender != "" {
		s += " (where: " + e.Sender + ")"
	}
	if e.Filename != "" {
		s += " in " + e.Filename
	}
	if e.Line > 0 {
		s += fmt.Sprintf(" | Line %d Col %d", e.Line, e.Column)
		if e.Token != nil {
			s += fmt.Sprintf(" near '%s'", e.Token.Val)
		}
	}
	s += "] "
	s += e.OrigError.Error()
	return s
}

// RawLine returns the affected line from the original template, if available.
func (e *Error) RawLine() (line string, available bool, outErr error) {
	if e.Line <= 0 || e.Filename == "<string>" {
		return "", false, nil
	}

	filename := e.Filename
	if e.Template != nil {
		filename = e.Template.set.resolveFilename(e.Template, e.Filename)
	}
	file, err := os.Open(filename)
	if err != nil {
		return "", false, err
	}
	defer func() {
		err := file.Close()
		if err != nil && outErr == nil {
			outErr = err
		}
	}()

	scanner := bufio.NewScanner(file)
	l := 0
	for scanner.Scan() {
		l++
		if l == e.Line {
			return scanner.Text(), true, nil
		}
	}
	return "", false, nil
}
