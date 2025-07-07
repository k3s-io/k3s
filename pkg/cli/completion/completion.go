package completion

import (
	"fmt"
	"os"

	"github.com/k3s-io/k3s/pkg/version"

	"github.com/urfave/cli/v2"
)

var (
	bashScript = `#!/usr/bin/env bash
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
`

	zshScript = `#compdef %[1]s
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

compdef _cli_zsh_autocomplete %[1]s`
)

func Bash(ctx *cli.Context) error {
	completetionScript, err := genCompletionScript(bashScript)
	if err != nil {
		return err
	}
	if ctx.Bool("i") {
		return writeToRC("bash", "/.bashrc")
	}
	fmt.Println(completetionScript)
	return nil
}

func Zsh(ctx *cli.Context) error {
	completetionScript, err := genCompletionScript(zshScript)
	if err != nil {
		return err
	}
	if ctx.Bool("i") {
		return writeToRC("zsh", "/.zshrc")
	}
	fmt.Println(completetionScript)
	return nil
}

func genCompletionScript(script string) (string, error) {
	completionScript := fmt.Sprintf(script, version.Program)
	return completionScript, nil
}

func writeToRC(shell, envFileName string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}

	envFileName = home + envFileName
	f, err := os.OpenFile(envFileName, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}

	defer f.Close()
	shellEntry := fmt.Sprintf("# >> %[1]s command completion (start)\n. <(%[1]s completion %s)\n# >> %[1]s command completion (end)", version.Program, shell)
	if _, err := f.WriteString(shellEntry); err != nil {
		return err
	}

	fmt.Printf("Autocomplete for %s added to: %s\n", shell, envFileName)
	return nil
}
