package types

import (
	"encoding/json"
	"io"
	"regexp"

	"github.com/ghodss/yaml"
)

var (
	commenter = regexp.MustCompile("(?m)^( *)zzz#\\((.*)\\)\\((.*)\\)([a-z]+.*):(.*)")
)

func JSONEncoder(writer io.Writer, v interface{}) error {
	return json.NewEncoder(writer).Encode(v)
}

func YAMLEncoder(writer io.Writer, v interface{}) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	buf, err := yaml.JSONToYAML(data)
	if err != nil {
		return err
	}
	//buf = commenter.ReplaceAll(buf, []byte("${1}# ${2}type: ${3}\n${1}# ${4}:${5}"))
	buf = commenter.ReplaceAll(buf, []byte("${1}# ${4}:${5}"))
	_, err = writer.Write(buf)
	return err
}
