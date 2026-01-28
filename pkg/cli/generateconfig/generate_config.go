package generateconfig

import (
	"fmt"
	"io"
	"os"
	"strings"
	"text/template"

	"github.com/k3s-io/k3s/pkg/cli/cmds"
	"github.com/k3s-io/k3s/pkg/version"
	"github.com/urfave/cli/v2"
	"gopkg.in/yaml.v3"
)

const yamlHeader = `{{- /* */ -}}
# Example {{ .Program }} {{ .Command }} configuration file
# This file contains all available configuration options with their descriptions.
# Uncomment and modify the options you want to use.
# Place this file at /etc/rancher/{{ .Program }}/config.yaml or use --config to specify a different location.
#

`

type cmdInfo struct {
	Program string
	Command string
}

// Run generates an example k3s config file
func Run(ctx *cli.Context) error {
	configType := cmds.GenerateConfigConfig.ConfigType
	output := cmds.GenerateConfigConfig.Output

	if configType != "server" && configType != "agent" {
		return fmt.Errorf("invalid config type %q, must be 'server' or 'agent'", configType)
	}

	var flags []cli.Flag
	if configType == "server" {
		flags = cmds.ServerFlags
	} else {
		flags = cmds.AgentFlags
	}

	var currentValues map[string]interface{}
	if configFile := cmds.GenerateConfigConfig.FromConfig; configFile != "" {
		if data, err := os.ReadFile(configFile); err == nil {
			currentValues = make(map[string]interface{})
			_ = yaml.Unmarshal(data, &currentValues)
		}
	}

	yamlContent, err := generateYAMLWithComments(flags, currentValues, configType)
	if err != nil {
		return err
	}

	var writer io.Writer = os.Stdout
	if output != "" {
		file, err := os.Create(output)
		if err != nil {
			return fmt.Errorf("failed to create output file: %w", err)
		}
		defer file.Close()
		writer = file
	}

	_, err = writer.Write([]byte(yamlContent))
	return err
}

// generateYAMLWithComments creates a YAML string with comments
func generateYAMLWithComments(flags []cli.Flag, currentValues map[string]interface{}, configType string) (string, error) {
	var sb strings.Builder

	tmpl, err := template.New("config").Parse(yamlHeader)
	if err != nil {
		return "", err
	}
	if err = tmpl.Execute(&sb, cmdInfo{Program: version.Program, Command: configType}); err != nil {
		return "", err
	}

	categories := groupFlagsByCategory(flags)

	for _, category := range []string{"listener", "cluster", "client", "data", "networking", "agent/node", "agent/networking", "agent/runtime", "flags", "db", "secrets-encryption", "experimental", "components", "other"} {
		categoryFlags := categories[category]
		if len(categoryFlags) == 0 {
			continue
		}

		categoryName := strings.Title(strings.ReplaceAll(category, "/", " - "))
		if category == "other" {
			categoryName = "Other Options"
		}
		fmt.Fprintf(&sb, "# %s\n", strings.Repeat("=", 80))
		fmt.Fprintf(&sb, "# %s\n", categoryName)
		fmt.Fprintf(&sb, "# %s\n\n", strings.Repeat("=", 80))

		for _, flag := range categoryFlags {
			writeFlag(&sb, flag, currentValues)
		}

		sb.WriteString("\n")
	}

	return sb.String(), nil
}

// groupFlagsByCategory organizes flags into categories based on their usage text
func groupFlagsByCategory(flags []cli.Flag) map[string][]cli.Flag {
	categories := make(map[string][]cli.Flag)

	for _, flag := range flags {
		if isHiddenFlag(flag) {
			continue
		}

		switch getFlagName(flag) {
		case "config", "debug", "v", "vmodule", "log", "alsologtostderr":
			continue
		}

		usage := getFlagUsage(flag)
		category := extractCategory(usage)
		categories[category] = append(categories[category], flag)
	}

	return categories
}

// extractCategory extracts the category from usage text
func extractCategory(usage string) string {
	if idx := strings.Index(usage, "("); idx != -1 {
		if endIdx := strings.Index(usage[idx:], ")"); endIdx != -1 {
			return usage[idx+1 : idx+endIdx]
		}
	}
	return "other"
}

// writeFlag writes a single flag to the string builder with comments
func writeFlag(sb *strings.Builder, flag cli.Flag, currentValues map[string]interface{}) {
	name := getFlagName(flag)
	usage := getFlagUsage(flag)

	if idx := strings.Index(usage, ") "); idx != -1 {
		usage = usage[idx+2:]
	}

	fmt.Fprintf(sb, "# %s\n", usage)

	value := getFlagValue(flag, name, currentValues)
	yamlValue := formatYAMLValue(value, flag)
	fmt.Fprintf(sb, "# %s: %s\n\n", name, yamlValue)
}

// getFlagName extracts the name from a flag
func getFlagName(flag cli.Flag) string {
	names := flag.Names()
	if len(names) > 0 {
		return names[0]
	}
	return ""
}

// getFlagUsage extracts the usage text from a flag
func getFlagUsage(flag cli.Flag) string {
	if df, ok := flag.(cli.DocGenerationFlag); ok {
		return df.GetUsage()
	}
	return ""
}

// isHiddenFlag checks if a flag is hidden
func isHiddenFlag(flag cli.Flag) bool {
	if vf, ok := flag.(cli.VisibleFlag); ok {
		return !vf.IsVisible()
	}
	return false
}

// getFlagValue gets the current or default value for a flag
func getFlagValue(flag cli.Flag, name string, currentValues map[string]interface{}) interface{} {
	if currentValues != nil {
		if val, exists := currentValues[name]; exists {
			return val
		}
	}

	return getDefaultValue(flag)
}

// getDefaultValue extracts the default value from a flag using type assertions
func getDefaultValue(flag cli.Flag) interface{} {
	switch f := flag.(type) {
	case *cli.StringFlag:
		return f.Value
	case *cli.BoolFlag:
		return f.Value
	case *cli.IntFlag:
		return f.Value
	case *cli.Int64Flag:
		return f.Value
	case *cli.UintFlag:
		return f.Value
	case *cli.Uint64Flag:
		return f.Value
	case *cli.Float64Flag:
		return f.Value
	case *cli.DurationFlag:
		return f.Value.String()
	case *cli.StringSliceFlag:
		if f.Value != nil {
			return f.Value.Value()
		}
		return []string{}
	case *cli.IntSliceFlag:
		if f.Value != nil {
			return f.Value.Value()
		}
		return []int{}
	default:
		return nil
	}
}

// formatYAMLValue formats a value for YAML output
func formatYAMLValue(value interface{}, flag cli.Flag) string {
	if value == nil {
		return "\"\""
	}

	switch v := value.(type) {
	case string:
		if v == "" {
			return "\"\""
		}
		if strings.Contains(v, " ") || strings.Contains(v, ":") || strings.Contains(v, "#") {
			return fmt.Sprintf("%q", v)
		}
		return v
	case bool:
		return fmt.Sprintf("%t", v)
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return fmt.Sprintf("%v", v)
	case []string:
		if len(v) == 0 {
			return "[]"
		}
		parts := make([]string, len(v))
		for i, s := range v {
			if strings.Contains(s, " ") || strings.Contains(s, ":") {
				parts[i] = fmt.Sprintf("%q", s)
			} else {
				parts[i] = s
			}
		}
		return "[" + strings.Join(parts, ", ") + "]"
	case cli.StringSlice:
		return formatYAMLValue(v.Value(), flag)
	case []int:
		if len(v) == 0 {
			return "[]"
		}
		parts := make([]string, len(v))
		for i, n := range v {
			parts[i] = fmt.Sprintf("%d", n)
		}
		return "[" + strings.Join(parts, ", ") + "]"
	default:
		data, err := yaml.Marshal(value)
		if err != nil {
			return fmt.Sprintf("%v", value)
		}
		return strings.TrimSpace(string(data))
	}
}
