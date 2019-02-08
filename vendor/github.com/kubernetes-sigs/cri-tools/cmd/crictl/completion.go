/*
Copyright 2017 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package crictl

import (
	"fmt"
	"strings"

	"github.com/urfave/cli"
)

var bashCompletionTemplate = `_cli_bash_autocomplete() {
    local cur opts base
    COMPREPLY=()
    cur="${COMP_WORDS[COMP_CWORD]}"
    opts="%s"
    COMPREPLY=( $(compgen -W "${opts}" -- ${cur}) )
    return 0
}

complete -F _cli_bash_autocomplete crictl`

var completionCommand = cli.Command{
	Name:  "completion",
	Usage: "Output bash shell completion code",
	Description: `Output bash shell completion code.

Examples:

    # Installing bash completion on Linux
    source <(crictl completion)
	`,
	Action: func(c *cli.Context) error {
		subcommands := []string{}
		for _, command := range c.App.Commands {
			if command.Hidden {
				continue
			}
			for _, name := range command.Names() {
				subcommands = append(subcommands, name)
			}
		}

		for _, flag := range c.App.Flags {
			// only includes full flag name.
			subcommands = append(subcommands, "--"+strings.Split(flag.GetName(), ",")[0])
		}

		fmt.Fprintln(c.App.Writer, fmt.Sprintf(bashCompletionTemplate, strings.Join(subcommands, "\n")))
		return nil
	},
}
