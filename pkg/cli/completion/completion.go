package completion

import (
	"fmt"
	"os"
)

func ShellCompletionInstall(appName, shell string) error {
	completetionScript, err := genCompletionScript(appName, shell)
	if err != nil {
		return err
	}
	autoFile := "/etc/rancher/" + appName + "/autocomplete"
	if err := os.WriteFile(autoFile, []byte(completetionScript), 0644); err != nil {
		return err
	}
	return writeToRC(appName, shell)
}

func genCompletionScript(appName, shell string) (string, error) {
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
		return "", fmt.Errorf("unkown shell: %s", shell)
	}

	return completionScript, nil
}

func writeToRC(appName, shell string) error {
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
	bashEntry := fmt.Sprintf("# >> %[1]s command completion (start)\n. /etc/rancher/%[1]s/autocomplete\n# >> %[1]s command completion (end)", appName)
	if _, err := f.WriteString(bashEntry); err != nil {
		return err
	}
	fmt.Println("Autocomplete installed in: ", rcFileName)
	return nil
}
