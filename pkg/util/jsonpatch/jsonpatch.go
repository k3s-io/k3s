package jsonpatch

import (
	"encoding/json"
	"strings"

	"k8s.io/apimachinery/pkg/labels"
)

// PatchBuilder is an interface for building a set of changes to a JSON document.
type PatchBuilder interface {
	Add(value interface{}, path ...string) PatchBuilder
	Remove(path ...string) PatchBuilder
	Replace(value interface{}, path ...string) PatchBuilder
	Copy(from []string, path ...string) PatchBuilder
	Move(from []string, path ...string) PatchBuilder
	Test(value interface{}, path ...string) PatchBuilder

	AddIfNotEqual(ls labels.Set, key, value string) PatchBuilder
	RemoveIfHas(ls labels.Set, key string) PatchBuilder

	WithPath(path ...string) PatchBuilder

	Marshal() ([]byte, error)
	MustMarshal() []byte

	Len() int
}

type operation struct {
	Op    string      `json:"op"`
	Path  string      `json:"path"`
	Value interface{} `json:"value,omitempty"`
	From  string      `json:"from,omitempty"`
}

type patch struct {
	ops  *[]operation
	base []string
}

// Explicit interface check
var _ PatchBuilder = &patch{}

// NewBuilder returns a new builder that can be used to create a chain of operations.
func NewBuilder(path ...string) PatchBuilder {
	return &patch{
		ops:  &[]operation{},
		base: append([]string{""}, path...),
	}
}

func (p *patch) Add(value interface{}, path ...string) PatchBuilder {
	path = append(p.base, path...)
	*p.ops = append(*p.ops, operation{Op: "add", Value: value, Path: encodePath(path)})
	return p
}

func (p *patch) Remove(path ...string) PatchBuilder {
	path = append(p.base, path...)
	*p.ops = append(*p.ops, operation{Op: "remove", Path: encodePath(path)})
	return p
}

func (p *patch) Replace(value interface{}, path ...string) PatchBuilder {
	path = append(p.base, path...)
	*p.ops = append(*p.ops, operation{Op: "replace", Value: value, Path: encodePath(path)})
	return p
}

func (p *patch) Copy(from []string, path ...string) PatchBuilder {
	from = append(p.base, from...)
	path = append(p.base, path...)
	*p.ops = append(*p.ops, operation{Op: "copy", From: encodePath(from), Path: encodePath(path)})
	return p
}

func (p *patch) Move(from []string, path ...string) PatchBuilder {
	from = append(p.base, from...)
	path = append(p.base, path...)
	*p.ops = append(*p.ops, operation{Op: "move", From: encodePath(from), Path: encodePath(path)})
	return p
}

func (p *patch) Test(value interface{}, path ...string) PatchBuilder {
	path = append(p.base, path...)
	*p.ops = append(*p.ops, operation{Op: "test", Value: value, Path: encodePath(path)})
	return p
}

func (p *patch) AddIfNotEqual(ls labels.Set, key, value string) PatchBuilder {
	if ls.Get(key) != value {
		p.Add(value, key)
	}
	return p
}

func (p *patch) RemoveIfHas(ls labels.Set, key string) PatchBuilder {
	if ls.Has(key) {
		p.Remove(key)
	}
	return p
}

func (p *patch) WithPath(path ...string) PatchBuilder {
	return &patch{
		ops:  p.ops,
		base: append(p.base, path...),
	}
}

func (p *patch) Marshal() ([]byte, error) {
	return json.Marshal(p.ops)
}

func (p *patch) MustMarshal() []byte {
	b, err := p.Marshal()
	if err != nil {
		panic("Failed to marshal JSON patch: " + err.Error())
	}
	return b
}

func (p *patch) Len() int {
	return len(*p.ops)
}

// encodePath encodes the two characters reserved by RFC6901, and joins the path components together.
func encodePath(path []string) string {
	for i := range path {
		path[i] = strings.ReplaceAll(path[i], "~", "~0")
		path[i] = strings.ReplaceAll(path[i], "/", "~1")
	}
	return strings.Join(path, "/")
}
