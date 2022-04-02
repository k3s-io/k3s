package cmds

import (
	"fmt"

	"github.com/urfave/cli"
)

func genShellCompletion(shell string) error {
	var completionScript string
	if shell == "bash" {
		completionScript = fmt.Sprintf(`#! /bin/bash
_cli_bash_autocomplete() {
if [[ "${COMP_WORDS[0]}" != "source" ]]; then
	local cur opts base
	COMPREPLY=()
	cur="${COMP_WORDS[COMP_CWORD]}"
	if [[ "$cur" == "-"* ]]; then
	opts=$( ${COMP_WORDS[@]:0:$COMP_CWORD} ${cur} --generate-bash-completion )
	else
	opts=$( ${COMP_WORDS[@]:0:$COMP_CWORD} --generate-bash-completion )
	fi
	COMPREPLY=( $(compgen -W "${opts}" -- ${cur}) )
	return 0
fi
}

complete -o bashdefault -o default -o nospace -F _cli_bash_autocomplete %s
`, appName)
	} else if shell == "zsh" {
		completionScript = fmt.Sprintf(`#compdef %[1]s
_cli_zsh_autocomplete() {

	local -a opts
	local cur
	cur=${words[-1]}
	if [[ "$cur" == "-"* ]]; then
	opts=("${(@f)$(_CLI_ZSH_AUTOCOMPLETE_HACK=1 ${words[@]:0:#words[@]-1} ${cur} --generate-bash-completion)}")
	else
	opts=("${(@f)$(_CLI_ZSH_AUTOCOMPLETE_HACK=1 ${words[@]:0:#words[@]-1} --generate-bash-completion)}")
	fi

	if [[ "${opts[1]}" != "" ]]; then
	_describe 'values' opts
	else
	_files
	fi

	return
}

compdef _cli_zsh_autocomplete %[1]s`, appName)
	} else {
		return fmt.Errorf("unkown shell, ")
	}

	fmt.Println(completionScript)
	return nil
}

func NewCompletionCommand() cli.Command {
	return cli.Command{
		Name:      "completion",
		Usage:     "Generate shell completion script",
		UsageText: appName + " completion [SHELL] (valid shells: bash, zsh)",
		Action: func(ctx *cli.Context) error {
			if ctx.NArg() < 1 {
				return fmt.Errorf("must provide a valid SHELL argument")
			}
			genShellCompletion(ctx.Args()[0])
			return nil
		},
	}
}
