package cli

import (
	"fmt"
	"io/ioutil"
	"reflect"
	"regexp"
	"runtime"
	"strings"

	"github.com/rancher/spur/flag"
	"github.com/rancher/spur/generic"
)

// Flag is a common interface related to parsing flags in cli.
type Flag interface {
	// Apply Flag settings to the given flag set
	Apply(*flag.FlagSet) error
}

// BashCompletionFlag enables bash-completion for all commands and subcommands
var BashCompletionFlag Flag = &BoolFlag{
	Name:   "generate-bash-completion",
	Hidden: true,
}

// VersionFlag prints the version for the application
var VersionFlag Flag = &BoolFlag{
	Name:    "version",
	Aliases: []string{"v"},
	Usage:   "print the version",
}

// HelpFlag prints the help for all commands and subcommands.
// Set to nil to disable the flag.  The subcommand
// will still be added unless HideHelp or HideHelpCommand is set to true.
var HelpFlag Flag = &BoolFlag{
	Name:    "help",
	Aliases: []string{"h"},
	Usage:   "show help",
}

// FlagStringer converts a flag definition to a string. This is used by help
// to display a flag.
var FlagStringer FlagStringFunc = stringifyFlag

// FlagNamePrefixer converts a full flag name and its placeholder into the help
// message flag prefix. This is used by the default FlagStringer.
var FlagNamePrefixer FlagNamePrefixFunc = prefixedNames

// FlagEnvHinter annotates flag help message with the environment variable
// details. This is used by the default FlagStringer.
var FlagEnvHinter FlagEnvHintFunc = withEnvHint

// FlagFileHinter annotates flag help message with the environment variable
// details. This is used by the default FlagStringer.
var FlagFileHinter FlagFileHintFunc = withFileHint

// FlagsByName is a slice of Flag.
type FlagsByName []Flag

const defaultPlaceholder = "value"

func (f FlagsByName) Len() int {
	return len(f)
}

func (f FlagsByName) Less(i, j int) bool {
	namesI := FlagNames(f[i])
	namesJ := FlagNames(f[j])
	if len(namesJ) == 0 {
		return false
	} else if len(namesI) == 0 {
		return true
	}
	return lexicographicLess(namesI[0], namesJ[0])
}

func (f FlagsByName) Swap(i, j int) {
	f[i], f[j] = f[j], f[i]
}

func flagSet(name string, flags []Flag) (*flag.FlagSet, error) {
	set := flag.NewFlagSet(name, flag.ContinueOnError)

	for _, f := range flags {
		if err := f.Apply(set); err != nil {
			return nil, err
		}
	}
	set.SetOutput(ioutil.Discard)
	return set, nil
}

func visibleFlags(fl []Flag) []Flag {
	var visible []Flag
	for _, f := range fl {
		if hidden, ok := getFlagHidden(f); !hidden || !ok {
			visible = append(visible, f)
		}
	}
	return visible
}

func prefixFor(name string) (prefix string) {
	if len(name) == 1 {
		prefix = "-"
	} else {
		prefix = "--"
	}

	return
}

// Returns the placeholder, if any, and the unquoted usage string.
func unquoteUsage(usage string) (string, string) {
	for i := 0; i < len(usage); i++ {
		if usage[i] == '`' {
			for j := i + 1; j < len(usage); j++ {
				if usage[j] == '`' {
					name := usage[i+1 : j]
					usage = usage[:i] + name + usage[j+1:]
					return name, usage
				}
			}
			break
		}
	}
	return "", usage
}

func prefixedNames(names []string, placeholder string) string {
	var prefixed string
	for i, name := range names {
		if name == "" {
			continue
		}

		prefixed += prefixFor(name) + name
		if placeholder != "" {
			prefixed += " " + placeholder
		}
		if i < len(names)-1 {
			prefixed += ", "
		}
	}
	return prefixed
}

func withEnvHint(envVars []string, str string) string {
	envText := ""
	if envVars != nil && len(envVars) > 0 {
		prefix := "$"
		suffix := ""
		sep := ", $"
		if runtime.GOOS == "windows" {
			prefix = "%"
			suffix = "%"
			sep = "%, %"
		}

		envText = fmt.Sprintf(" [%s%s%s]", prefix, strings.Join(envVars, sep), suffix)
	}
	return str + envText
}

// FlagNames returns the name and aliases for a given flag, and panics
// if any of the values are invalid
func FlagNames(f Flag) []string {
	name, ok := getFlagName(f)
	if !ok {
		panic("flag is missing name field")
	}
	aliases, _ := getFlagAliases(f)

	var ret []string

	for _, part := range strings.Split(name, ",") {
		// urfave/cli v1 -> v2 migration warning zone:
		// split name as per v1 standard
		ret = append(ret, strings.TrimSpace(part))
	}

	// add the aliases to our names
	ret = append(ret, aliases...)

	// validate the names and panic on failure
	for _, part := range ret {
		if strings.Contains(part, ",") {
			panic(fmt.Errorf("flag name contains a comma: %q", part))
		}
		if regexp.MustCompile(`\s`).Match([]byte(part)) {
			panic(fmt.Errorf("flag name contains whitespace: %q", part))
		}
		if part == "" {
			panic("flag has an empty name")
		}
	}

	return ret
}

func flagStringSliceField(f Flag, name string) []string {
	fv := flagValue(f)
	field := fv.FieldByName(name)

	if field.IsValid() {
		return field.Interface().([]string)
	}

	return []string{}
}

func withFileHint(filePath, str string) string {
	fileText := ""
	if filePath != "" {
		fileText = fmt.Sprintf(" [%s]", filePath)
	}
	return str + fileText
}

func flagValue(f Flag) reflect.Value {
	fv := reflect.ValueOf(f)
	for fv.Kind() == reflect.Ptr {
		fv = reflect.Indirect(fv)
	}
	return fv
}

func formatDefault(format string) string {
	return " (default: " + format + ")"
}

func stringifyFlag(f Flag) string {
	value, _ := getFlagValue(f)
	usage, _ := getFlagUsage(f)

	if generic.IsSlice(value) {
		return withEnvHint(flagStringSliceField(f, "EnvVars"),
			stringifySlice(usage, FlagNames(f), value))
	}

	placeholder, usage := unquoteUsage(usage)

	needsPlaceholder := false
	defaultValueString := ""

	var valKind reflect.Kind

	if valType := generic.TypeOf(value); valType != nil {
		valKind = valType.Kind()
		needsPlaceholder = valKind != reflect.Bool
	}

	defaultValueString = fmt.Sprintf(formatDefault("%v"), value)
	if valKind == reflect.String && value.(string) != "" {
		defaultValueString = fmt.Sprintf(formatDefault("%q"), value)
	}

	if helpText, ok := getFlagDefaultText(f); ok && helpText != "" {
		defaultValueString = fmt.Sprintf(formatDefault("%s"), helpText)
	}

	if defaultValueString == formatDefault("") {
		defaultValueString = ""
	}

	if needsPlaceholder && placeholder == "" {
		placeholder = defaultPlaceholder
	}

	usageWithDefault := strings.TrimSpace(usage + defaultValueString)

	return withEnvHint(flagStringSliceField(f, "EnvVars"),
		fmt.Sprintf("%s\t%s", prefixedNames(FlagNames(f), placeholder), usageWithDefault))
}

func stringifySlice(usage string, names []string, value interface{}) string {
	var defaults []string
	for i := 0; i < generic.Len(value); i++ {
		v := generic.Index(value, i)
		s, ok := v.(string)
		if ok && s == "" {
			continue
		}
		if ok {
			s = fmt.Sprintf("%q", s)
		} else {
			s, _ = generic.ToString(v)
		}
		defaults = append(defaults, s)
	}
	return stringifySliceFlag(usage, names, defaults)
}

func stringifySliceFlag(usage string, names, defaultVals []string) string {
	placeholder, usage := unquoteUsage(usage)
	if placeholder == "" {
		placeholder = defaultPlaceholder
	}

	defaultVal := ""
	if len(defaultVals) > 0 {
		defaultVal = fmt.Sprintf(formatDefault("%s"), strings.Join(defaultVals, ", "))
	}

	usageWithDefault := strings.TrimSpace(fmt.Sprintf("%s%s", usage, defaultVal))
	return fmt.Sprintf("%s\t%s", prefixedNames(names, placeholder), usageWithDefault)
}

func hasFlag(flags []Flag, fl Flag) bool {
	for _, existing := range flags {
		if fl == existing {
			return true
		}
	}
	return false
}
