package configfilearg

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/k3s-io/k3s/pkg/agent/util"
	"github.com/rancher/wrangler/v3/pkg/data/convert"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
	"gopkg.in/yaml.v2"
)

type Parser struct {
	After         []string
	ConfigFlags   []string
	OverrideFlags []string
	EnvName       string
	DefaultConfig string
	// ValidFlags are maps of flags that are valid for that particular conmmand. This enables us to ignore flags in
	// the config file that do no apply to the current command.
	ValidFlags map[string][]cli.Flag
}

// Parse will parse an os.Args style slice looking for Parser.FlagNames after Parse.After.
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

	if configFile := p.findConfigFileFlag(args); configFile != "" {
		values, err := readConfigFile(configFile)
		if err != nil {
			if os.IsNotExist(err) {
				return args, nil
			}
			return nil, err
		}
		if len(args) > 1 {
			values, err = p.stripInvalidFlags(args[1], values)
			if err != nil {
				return nil, err
			}
		}
		return append(prefix, append(values, suffix...)...), nil
	}

	return args, nil
}

func (p *Parser) stripInvalidFlags(command string, args []string) ([]string, error) {
	var result []string
	var cmdFlags []cli.Flag
	for k, v := range p.ValidFlags {
		if k == command {
			cmdFlags = v
		}
	}
	if len(cmdFlags) == 0 {
		return args, nil
	}
	validFlags := make(map[string]bool, len(cmdFlags))
	for _, f := range cmdFlags {
		//split flags with aliases into 2 entries
		for _, s := range strings.Split(f.GetName(), ",") {
			validFlags[s] = true
		}
	}

	re, err := regexp.Compile("^-+([^=]*)=")
	if err != nil {
		return args, err
	}
	for _, arg := range args {
		mArg := arg
		if match := re.FindAllStringSubmatch(arg, -1); match != nil {
			mArg = match[0][1]
		}
		if validFlags[mArg] {
			result = append(result, arg)
		} else {
			logrus.Warnf("Unknown flag %s found in config.yaml, skipping\n", strings.Split(arg, "=")[0])
		}
	}
	return result, nil
}

// FindString returns the string value of a flag, checking the CLI args,
// config file, and config file dropins. If the value is not found,
// an empty string is returned. It is not an error if no args,
// configfile, or dropins are present.
func (p *Parser) FindString(args []string, target string) (string, error) {
	// Check for --help or --version flags, which override any other flags
	if val, found := p.findOverrideFlag(args); found {
		return val, nil
	}

	var files []string
	var lastVal string

	if configFile := p.findConfigFileFlag(args); configFile != "" {
		if _, err := os.Stat(configFile); err == nil {
			files = append(files, configFile)
		}

		dropinFiles, err := dotDFiles(configFile)
		if err != nil {
			return "", err
		}
		files = append(files, dropinFiles...)
	}

	for _, file := range files {
		bytes, err := readConfigFileData(file)
		if err != nil {
			return "", err
		}

		data := yaml.MapSlice{}
		if err := yaml.Unmarshal(bytes, &data); err != nil {
			return "", err
		}
		for _, i := range data {
			k, v := convert.ToString(i.Key), convert.ToString(i.Value)
			isAppend := strings.HasSuffix(k, "+")
			k = strings.TrimSuffix(k, "+")
			if k == target {
				if isAppend {
					lastVal = lastVal + "," + v
				} else {
					lastVal = v
				}
			}
		}
	}
	return lastVal, nil
}

func (p *Parser) findOverrideFlag(args []string) (string, bool) {
	for _, arg := range args {
		for _, flagName := range p.OverrideFlags {
			if flagName == arg {
				return arg, true
			}
		}
	}

	return "", false
}

// findConfigFileFlag returns the value of the config file env var or CLI flag.
// If neither are set, it returns the default value.
func (p *Parser) findConfigFileFlag(args []string) string {
	if envVal := os.Getenv(p.EnvName); p.EnvName != "" && envVal != "" {
		return envVal
	}

	for i, arg := range args {
		for _, flagName := range p.ConfigFlags {
			if flagName == arg {
				if len(args) > i+1 {
					return args[i+1]
				}
				// This is actually invalid, so we rely on the CLI parser after the fact flagging it as bad
				return ""
			} else if strings.HasPrefix(arg, flagName+"=") {
				return arg[len(flagName)+1:]
			}
		}
	}

	return p.DefaultConfig
}

func (p *Parser) findStart(args []string) ([]string, []string, bool) {
	if len(p.After) == 0 {
		return []string{}, args, true
	}
	afterTemp := append([]string{}, p.After...)
	afterIndex := make(map[string]int)
	re, err := regexp.Compile(`(.+):(\d+)`)
	if err != nil {
		return args, nil, false
	}
	// After keywords ending with ":<NUM>" can set + NUM of arguments as the split point.
	// used for matching on subcommmands
	for i, arg := range afterTemp {
		if match := re.FindAllStringSubmatch(arg, -1); match != nil {
			afterTemp[i] = match[0][1]
			afterIndex[match[0][1]], err = strconv.Atoi(match[0][2])
			if err != nil {
				return args, nil, false
			}
		}
	}

	for i, val := range args {
		for _, test := range afterTemp {
			if val == test {
				if skip := afterIndex[test]; skip != 0 {
					if len(args) <= i+skip || strings.HasPrefix(args[i+skip], "-") {
						return args[0 : i+1], args[i+1:], true
					}
					return args[0 : i+skip+1], args[i+skip+1:], true
				}
				return args[0 : i+1], args[i+1:], true
			}
		}
	}
	return args, nil, false
}

func dotDFiles(basefile string) (result []string, _ error) {
	files, err := os.ReadDir(basefile + ".d")
	if os.IsNotExist(err) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	for _, file := range files {
		if file.IsDir() || !util.HasSuffixI(file.Name(), ".yaml", ".yml") {
			continue
		}
		result = append(result, filepath.Join(basefile+".d", file.Name()))
	}
	return
}

// readConfigFile returns a flattened arg list generated from the specified config
// file, and any config file dropins in the dropin directory that corresponds to that
// config file.  The config file or at least one dropin must exist.
func readConfigFile(file string) (result []string, _ error) {
	files, err := dotDFiles(file)
	if err != nil {
		return nil, err
	}

	if _, err = os.Stat(file); err != nil {
		// If the config file doesn't exist and we have dropins that's fine.
		// Other errors are bubbled up regardless of how many dropins we have.
		if !(os.IsNotExist(err) && len(files) > 0) {
			return nil, err
		}
	} else {
		// The config file exists, load it first.
		files = append([]string{file}, files...)
	}

	var (
		keySeen  = map[string]bool{}
		keyOrder []string
		values   = map[string]interface{}{}
	)
	for _, file := range files {
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
			isAppend := strings.HasSuffix(k, "+")
			k = strings.TrimSuffix(k, "+")

			if !keySeen[k] {
				keySeen[k] = true
				keyOrder = append(keyOrder, k)
			}

			if oldValue, ok := values[k]; ok && isAppend {
				values[k] = append(toSlice(oldValue), toSlice(v)...)
			} else {
				values[k] = v
			}
		}
	}

	for _, k := range keyOrder {
		v := values[k]

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

func toSlice(v interface{}) []interface{} {
	switch k := v.(type) {
	case string:
		return []interface{}{k}
	case []interface{}:
		return k
	default:
		str := strings.TrimSpace(convert.ToString(v))
		if str == "" {
			return nil
		}
		return []interface{}{str}
	}
}

// readConfigFileData returns the contents of a local or remote file
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
		return io.ReadAll(resp.Body)
	default:
		return os.ReadFile(file)
	}
}
