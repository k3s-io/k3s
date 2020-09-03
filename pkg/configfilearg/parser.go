package configfilearg

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/rancher/wrangler/pkg/data/convert"
	"gopkg.in/yaml.v2"
)

type Parser struct {
	After         []string
	FlagNames     []string
	EnvName       string
	DefaultConfig string
}

// Parser will parse an os.Args style slice looking for Parser.FlagNames after Parse.After.
// It will read the parameter value of Parse.FlagNames and read the file, appending all flags directly after
// the Parser.After value. This means a the non-config file flags will override, or if a slice append to, the config
// file values.
// If Parser.DefaultConfig is set, the existence of the config file is optional if not set in the os.Args. This means
// if Parser.DefaultConfig is set we will always try to read the config file but only fail if it's not found if the
// args contains Parser.FlagNames
func (p *Parser) Parse(args []string) ([]string, error) {
	prefix, suffix, found := p.findStart(args)
	if !found {
		return args, nil
	}

	configFile, isSet := p.findConfigFileFlag(args)
	if configFile != "" {
		values, err := readConfigFile(configFile)
		if !isSet && os.IsNotExist(err) {
			return args, nil
		} else if err != nil {
			return nil, err
		}
		return append(prefix, append(values, suffix...)...), nil
	}

	return args, nil
}

func (p *Parser) findConfigFileFlag(args []string) (string, bool) {
	if envVal := os.Getenv(p.EnvName); p.EnvName != "" && envVal != "" {
		return envVal, true
	}

	for i, arg := range args {
		for _, flagName := range p.FlagNames {
			if flagName == arg {
				if len(args) > i+1 {
					return args[i+1], true
				}
				// This is actually invalid, so we rely on the CLI parser after the fact flagging it as bad
				return "", false
			} else if strings.HasPrefix(arg, flagName+"=") {
				return arg[len(flagName)+1:], true
			}
		}
	}

	return p.DefaultConfig, false
}

func (p *Parser) findStart(args []string) ([]string, []string, bool) {
	if len(p.After) == 0 {
		return []string{}, args, true
	}

	for i, val := range args {
		for _, test := range p.After {
			if val == test {
				return args[0 : i+1], args[i+1:], true
			}
		}
	}

	return args, nil, false
}

func readConfigFile(file string) (result []string, _ error) {
	bytes, err := readConfigFileData(file)
	if err != nil {
		return nil, err
	}

	data := yaml.MapSlice{}
	if err := yaml.Unmarshal(bytes, &data); err != nil {
		return nil, err
	}

	for _, i := range data {
		k, v := convert.ToString(i.Key), i.Value
		prefix := "--"
		if len(k) == 1 {
			prefix = "-"
		}

		if slice, ok := v.([]interface{}); ok {
			for _, v := range slice {
				result = append(result, prefix+k+"="+convert.ToString(v))
			}
		} else {
			str := convert.ToString(v)
			result = append(result, prefix+k+"="+str)
		}
	}

	return
}

func readConfigFileData(file string) ([]byte, error) {
	u, err := url.Parse(file)
	if err != nil {
		return nil, fmt.Errorf("failed to parse config location %s: %w", file, err)
	}

	switch u.Scheme {
	case "http", "https":
		resp, err := http.Get(file)
		if err != nil {
			return nil, fmt.Errorf("failed to read http config %s: %w", file, err)
		}
		defer resp.Body.Close()
		return ioutil.ReadAll(resp.Body)
	default:
		return ioutil.ReadFile(file)
	}
}
