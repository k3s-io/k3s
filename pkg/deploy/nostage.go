//go:build no_stage
// +build no_stage

package deploy

func Stage(dataDir string, templateVars map[string]string, skips map[string]bool) error {
	return nil
}
