package altsrc

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"

	"github.com/rancher/spur/cli"
	"gopkg.in/yaml.v2"
)

type yamlSourceContext struct {
	FilePath string
}

// NewYamlSourceFromFile creates a new Yaml cli.InputSourceContext from a filepath.
func NewYamlSourceFromFile(file string) (cli.InputSourceContext, error) {
	ysc := &yamlSourceContext{FilePath: file}
	var results map[interface{}]interface{}
	err := readCommandYaml(ysc.FilePath, &results)
	return &MapInputSource{file: file, valueMap: results}, err
}

func readCommandYaml(filePath string, container interface{}) error {
	b, err := loadDataFrom(filePath)
	if err != nil {
		return err
	}
	return yaml.Unmarshal(b, container)
}

func loadDataFrom(filePath string) ([]byte, error) {
	u, err := url.Parse(filePath)
	if err != nil {
		return nil, err
	}

	if u.Host != "" { // i have a host, now do i support the scheme?
		switch u.Scheme {
		case "http", "https":
			res, err := http.Get(filePath)
			if err != nil {
				return nil, err
			}
			return ioutil.ReadAll(res.Body)
		default:
			return nil, fmt.Errorf("scheme of %s is unsupported", filePath)
		}
	}
	if _, err := os.Stat(filePath); err != nil {
		return nil, os.ErrNotExist
	}
	return ioutil.ReadFile(filePath)
}
