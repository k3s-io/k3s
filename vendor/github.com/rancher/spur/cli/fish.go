package cli

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"text/template"

	"github.com/rancher/spur/flag"
)

// ToFishCompletion creates a fish completion string for the `*App`
// The function errors if either parsing or writing of the string fails.
func (a *App) ToFishCompletion() (string, error) {
	var w bytes.Buffer
	if err := a.writeFishCompletionTemplate(&w); err != nil {
		return "", err
	}
	return w.String(), nil
}

type fishCompletionTemplate struct {
	App         *App
	Completions []string
	AllCommands []string
}

func (a *App) writeFishCompletionTemplate(w io.Writer) error {
	const name = "cli"
	t, err := template.New(name).Parse(FishCompletionTemplate)
	if err != nil {
		return err
	}
	allCommands := []string{}

	// Add global flags
	completions := a.prepareFishFlags(a.VisibleFlags(), allCommands)

	// Add help flag
	if !a.HideHelp {
		completions = append(
			completions,
			a.prepareFishFlags([]Flag{HelpFlag}, allCommands)...,
		)
	}

	// Add version flag
	if !a.HideVersion {
		completions = append(
			completions,
			a.prepareFishFlags([]Flag{VersionFlag}, allCommands)...,
		)
	}

	// Add commands and their flags
	completions = append(
		completions,
		a.prepareFishCommands(a.VisibleCommands(), &allCommands, []string{})...,
	)

	return t.ExecuteTemplate(w, name, &fishCompletionTemplate{
		App:         a,
		Completions: completions,
		AllCommands: allCommands,
	})
}

func (a *App) prepareFishCommands(commands []*Command, allCommands *[]string, previousCommands []string) []string {
	completions := []string{}
	for _, command := range commands {
		if command.Hidden {
			continue
		}

		var completion strings.Builder
		completion.WriteString(fmt.Sprintf(
			"complete -r -c %s -n '%s' -a '%s'",
			a.Name,
			a.fishSubcommandHelper(previousCommands),
			strings.Join(command.Names(), " "),
		))

		if command.Usage != "" {
			completion.WriteString(fmt.Sprintf(" -d '%s'",
				escapeSingleQuotes(command.Usage)))
		}

		if !command.HideHelp {
			completions = append(
				completions,
				a.prepareFishFlags([]Flag{HelpFlag}, command.Names())...,
			)
		}

		*allCommands = append(*allCommands, command.Names()...)
		completions = append(completions, completion.String())
		completions = append(
			completions,
			a.prepareFishFlags(command.Flags, command.Names())...,
		)

		// recursevly iterate subcommands
		if len(command.Subcommands) > 0 {
			completions = append(
				completions,
				a.prepareFishCommands(
					command.Subcommands, allCommands, command.Names(),
				)...,
			)
		}
	}

	return completions
}

func (a *App) prepareFishFlags(flags []Flag, previousCommands []string) []string {
	completions := []string{}
	for _, f := range flags {
		completion := &strings.Builder{}
		completion.WriteString(fmt.Sprintf(
			"complete -c %s -n '%s'",
			a.Name,
			a.fishSubcommandHelper(previousCommands),
		))

		fishAddFileFlag(f, completion)

		for idx, opt := range FlagNames(f) {
			if idx == 0 {
				completion.WriteString(fmt.Sprintf(
					" -l %s", strings.TrimSpace(opt),
				))
			} else {
				completion.WriteString(fmt.Sprintf(
					" -s %s", strings.TrimSpace(opt),
				))

			}
		}

		if v, ok := getFlagValue(f); ok && !flag.IsBoolValue(v) {
			completion.WriteString(" -r")
		}

		if usage, ok := getFlagUsage(f); ok && usage != "" {
			completion.WriteString(fmt.Sprintf(" -d '%s'",
				escapeSingleQuotes(usage)))
		}

		completions = append(completions, completion.String())
	}

	return completions
}

func fishAddFileFlag(flag Flag, completion *strings.Builder) {
	if takesFile, ok := getFlagTakesFile(flag); ok && takesFile {
		return
	}
	completion.WriteString(" -f")
}

func (a *App) fishSubcommandHelper(allCommands []string) string {
	fishHelper := fmt.Sprintf("__fish_%s_no_subcommand", a.Name)
	if len(allCommands) > 0 {
		fishHelper = fmt.Sprintf(
			"__fish_seen_subcommand_from %s",
			strings.Join(allCommands, " "),
		)
	}
	return fishHelper

}

func escapeSingleQuotes(input string) string {
	return strings.Replace(input, `'`, `\'`, -1)
}
