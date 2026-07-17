//go:build windows

package flannel

import "os"

// no atomic writes on windows, fall back to os.WriteFile
// ref: https://github.com/google/renameio/blob/v2.0.2/README.md#windows-support
var writeFile = os.WriteFile
