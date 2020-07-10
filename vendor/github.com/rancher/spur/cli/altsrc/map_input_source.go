package altsrc

import (
	"strings"
)

// MapInputSource implements InputSourceContext to return
// data from the map that is loaded.
type MapInputSource struct {
	file     string
	valueMap map[interface{}]interface{}
}

// nestedVal checks if the name has '.' delimiters.
// If so, it tries to traverse the tree by the '.' delimited sections to find
// a nested value for the key.
func nestedVal(name string, tree map[interface{}]interface{}) (interface{}, bool) {
	if sections := strings.Split(name, "."); len(sections) > 1 {
		node := tree
		for _, section := range sections[:len(sections)-1] {
			child, ok := node[section]
			if !ok {
				return nil, false
			}
			ctype, ok := child.(map[interface{}]interface{})
			if !ok {
				return nil, false
			}
			node = ctype
		}
		if val, ok := node[sections[len(sections)-1]]; ok {
			return val, true
		}
	}
	return nil, false
}

// Get returns the named value
func (fsm *MapInputSource) Get(name string) (interface{}, bool) {
	if value, exists := fsm.valueMap[name]; exists {
		return value, true
	}
	return nestedVal(name, fsm.valueMap)
}

// Source returns the path of the source file
func (fsm *MapInputSource) Source() string {
	return fsm.file
}
