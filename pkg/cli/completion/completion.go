package completion

import (
	"fmt"
	"os"

	"github.com/k3s-io/k3s/pkg/version"

	"github.com/urfave/cli"
)

func Run(ctx *cli.Context) error {
	if ctx.NArg() < 1 {
		return fmt.Errorf("must provide a valid SHELL argument")
	}
	shell := ctx.Args()[0]
	completetionScript, err := genCompletionScript(shell)
	if err != nil {
		return err
	}
	if ctx.Bool("i") {
		return writeToRC(shell)
	}
	fmt.Println(completetionScript)
	return nil
}

func genCompletionScript(shell string) (string, error) {
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
`, version.Program)
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

compdef _cli_zsh_autocomplete %[1]s`, version.Program)
	} else {
		return "", fmt.Errorf("unknown shell: %s", shell)
	}

	return completionScript, nil
}

func writeToRC(shell string) error {
	rcFileName := ""
	if shell == "bash" {
		rcFileName = "/.bashrc"
	} else if shell == "zsh" {
		rcFileName = "/.zshrc"
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	rcFileName = home + rcFileName
	f, err := os.OpenFile(rcFileName, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	bashEntry := fmt.Sprintf("# >> %[1]s command completion (start)\n. <(%[1]s completion %s)\n# >> %[1]s command completion (end)", version.Program, shell)
	if _, err := f.WriteString(bashEntry); err != nil {
		return err
	}
	fmt.Printf("Autocomplete for %s added to: %s\n", shell, rcFileName)
	return nil
}
