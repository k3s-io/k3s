package altsrc

import (
	"fmt"
	"os"

	"github.com/rancher/spur/cli"
)

// NewConfigFromFlag creates a new Yaml cli.InputSourceContext from a provided flag name and source context.
// If the flag is not set and the default config does not exist then returns an empty input and no error.
func NewConfigFromFlag(flagFileName string) func(*cli.Context) (cli.InputSourceContext, error) {
	return func(ctx *cli.Context) (cli.InputSourceContext, error) {
		filePath := ctx.String(flagFileName)
		if isc, err := NewYamlSourceFromFile(filePath); ctx.IsSet(flagFileName) || !os.IsNotExist(err) {
			if err != nil {
				err = fmt.Errorf("unable to load config file '%s': %s", filePath, err)
			}
			return isc, err
		}
		return &MapInputSource{}, nil
	}
}
