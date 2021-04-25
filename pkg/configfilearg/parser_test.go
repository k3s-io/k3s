package configfilearg

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFindStart(t *testing.T) {
	testCases := []struct {
		input  []string
		prefix []string
		suffix []string
		found  bool
		what   string
	}{
		{
			input:  nil,
			prefix: nil,
			suffix: nil,
			found:  false,
			what:   "default case",
		},
		{
			input:  []string{"server"},
			prefix: []string{"server"},
			suffix: []string{},
			found:  true,
			what:   "simple case",
		},
		{
			input:  []string{"server", "foo"},
			prefix: []string{"server"},
			suffix: []string{"foo"},
			found:  true,
			what:   "also simple case",
		},
		{
			input:  []string{"server", "foo", "bar"},
			prefix: []string{"server"},
			suffix: []string{"foo", "bar"},
			found:  true,
			what:   "longer simple case",
		},
		{
			input:  []string{"not-server", "foo", "bar"},
			prefix: []string{"not-server", "foo", "bar"},
			found:  false,
			what:   "not found",
		},
	}

	p := Parser{
		After: []string{"server", "agent"},
	}

	for _, testCase := range testCases {
		prefix, suffix, found := p.findStart(testCase.input)
		assert.Equal(t, testCase.prefix, prefix)
		assert.Equal(t, testCase.suffix, suffix)
		assert.Equal(t, testCase.found, found)
	}
}

func TestConfigFile(t *testing.T) {
	testCases := []struct {
		input      []string
		env        string
		def        string
		configFile string
		found      bool
		what       string
	}{
		{
			input: nil,
			found: false,
			what:  "default case",
		},
		{
			input:      []string{"asdf", "-c", "value"},
			configFile: "value",
			found:      true,
			what:       "simple case",
		},
		{
			input: []string{"-c"},
			found: false,
			what:  "invalid args string",
		},
		{
			input: []string{"-c="},
			found: true,
			what:  "empty arg value",
		},
		{
			def:   "def",
			input: []string{"-c="},
			found: true,
			what:  "empty arg value override default",
		},
		{
			def:   "def",
			input: []string{"-c"},
			found: false,
			what:  "invalid args always return no value",
		},
		{
			def:        "def",
			input:      []string{"-c", "value"},
			configFile: "value",
			found:      true,
			what:       "value override default",
		},
		{
			def:        "def",
			configFile: "def",
			found:      false,
			what:       "default gets used when nothing is passed",
		},
		{
			def:        "def",
			input:      []string{"-c", "value"},
			env:        "env",
			configFile: "env",
			found:      true,
			what:       "env override args",
		},
		{
			def:        "def",
			input:      []string{"before", "-c", "value", "after"},
			configFile: "value",
			found:      true,
			what:       "garbage in start and end",
		},
	}

	for _, testCase := range testCases {
		p := Parser{
			FlagNames:     []string{"--config", "-c"},
			EnvName:       "_TEST_FLAG_ENV",
			DefaultConfig: testCase.def,
		}
		os.Setenv(p.EnvName, testCase.env)
		configFile, found := p.findConfigFileFlag(testCase.input)
		assert.Equal(t, testCase.configFile, configFile, testCase.what)
		assert.Equal(t, testCase.found, found, testCase.what)
	}
}

func TestParse(t *testing.T) {
	testDataOutput := []string{
		"--foo-bar=bar-foo",
		"--a-slice=1",
		"--a-slice=1.5",
		"--a-slice=2",
		"--a-slice=",
		"--a-slice=three",
		"--isempty=",
		"-c=b",
		"--isfalse=false",
		"--islast=true",
		"--b-string=one",
		"--b-string=two",
		"--c-slice=one",
		"--c-slice=two",
		"--c-slice=three",
		"--d-slice=three",
		"--d-slice=four",
		"--e-slice=one",
		"--e-slice=two",
	}

	defParser := Parser{
		After:         []string{"server", "agent"},
		FlagNames:     []string{"-c", "--config"},
		EnvName:       "_TEST_ENV",
		DefaultConfig: "./testdata/data.yaml",
	}

	testCases := []struct {
		parser Parser
		env    string
		input  []string
		output []string
		err    string
		what   string
	}{
		{
			parser: defParser,
			what:   "default case",
		},
		{
			parser: defParser,
			input:  []string{"server"},
			output: append([]string{"server"}, testDataOutput...),
			what:   "read config file when not specified",
		},
		{
			parser: Parser{
				After:         []string{"server", "agent"},
				FlagNames:     []string{"-c", "--config"},
				DefaultConfig: "missing",
			},
			input:  []string{"server"},
			output: []string{"server"},
			what:   "ignore missing config when not set",
		},
		{
			parser: Parser{
				After:         []string{"server", "agent"},
				FlagNames:     []string{"-c", "--config"},
				DefaultConfig: "missing",
			},
			input:  []string{"server", "-c=missing"},
			output: []string{"server", "-c=missing"},
			what:   "fail when missing config",
			err:    "stat missing: no such file or directory",
		},
		{
			parser: Parser{
				After:         []string{"server", "agent"},
				FlagNames:     []string{"-c", "--config"},
				DefaultConfig: "missing",
			},
			input:  []string{"before", "server", "before", "-c", "./testdata/data.yaml", "after"},
			output: append(append([]string{"before", "server"}, testDataOutput...), "before", "-c", "./testdata/data.yaml", "after"),
			what:   "read config file",
		},
		{
			parser: Parser{
				After:         []string{"server", "agent"},
				FlagNames:     []string{"-c", "--config"},
				DefaultConfig: "missing",
			},
			input: []string{"before", "server", "before", "-c", "./testdata/data.yaml.d/02-data.yaml", "after"},
			output: []string{"before", "server",
				"--foo-bar=bar-foo",
				"--b-string=two",
				"--c-slice=three",
				"--d-slice=three",
				"--d-slice=four",
				"--e-slice=one",
				"--e-slice=two",
				"before", "-c", "./testdata/data.yaml.d/02-data.yaml", "after"},
			what: "read single config file",
		},
	}

	for _, testCase := range testCases {
		os.Setenv(testCase.parser.EnvName, testCase.env)
		output, err := testCase.parser.Parse(testCase.input)
		if err == nil {
			assert.Equal(t, testCase.err, "", testCase.what)
		} else {
			assert.Equal(t, testCase.err, err.Error(), testCase.what)
		}
		if testCase.err == "" {
			assert.Equal(t, testCase.output, output, testCase.what)
		}
	}
}
