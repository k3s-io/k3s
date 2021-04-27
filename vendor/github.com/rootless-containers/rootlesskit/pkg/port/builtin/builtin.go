package builtin

import (
	"io"

	"github.com/rootless-containers/rootlesskit/pkg/port"
	"github.com/rootless-containers/rootlesskit/pkg/port/builtin/child"
	"github.com/rootless-containers/rootlesskit/pkg/port/builtin/parent"
)

var (
	NewParentDriver func(logWriter io.Writer, stateDir string) (port.ParentDriver, error) = parent.NewDriver
	NewChildDriver  func(logWriter io.Writer) port.ChildDriver                            = child.NewDriver
)
