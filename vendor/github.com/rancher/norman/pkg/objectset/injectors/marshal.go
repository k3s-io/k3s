package injectors

import (
	"bufio"
	"bytes"
	"io"

	"github.com/ghodss/yaml"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	yamlDecoder "k8s.io/apimachinery/pkg/util/yaml"
)

func ToBytes(objects []runtime.Object) ([]byte, error) {
	if len(objects) == 0 {
		return nil, nil
	}

	buffer := &bytes.Buffer{}
	for i, obj := range objects {
		if i > 0 {
			buffer.WriteString("\n---\n")
		}

		bytes, err := yaml.Marshal(obj)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to encode %s", obj.GetObjectKind().GroupVersionKind())
		}
		buffer.Write(bytes)
	}

	return buffer.Bytes(), nil
}

func FromBytes(content []byte) ([]runtime.Object, error) {
	var result []runtime.Object

	reader := yamlDecoder.NewYAMLReader(bufio.NewReader(bytes.NewBuffer(content)))
	for {
		raw, err := reader.Read()
		if err == io.EOF {
			break
		}

		data := map[string]interface{}{}
		if err := yaml.Unmarshal(raw, &data); err != nil {
			return nil, err
		}

		result = append(result, &unstructured.Unstructured{Object: data})
	}

	return result, nil
}
