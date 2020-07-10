package errors

import pkgErrors "github.com/pkg/errors"

var (
	// New is assigned to "github.com/pkg/errors".New by default
	New = pkgErrors.New
	// Cause is assigned to "github.com/pkg/errors".Cause by default
	Cause = pkgErrors.Cause
	// Errorf is assigned to "github.com/pkg/errors".Errorf by default
	Errorf = pkgErrors.Errorf
	// Annotate is assigned to "github.com/pkg/errors".Wrap by default
	Annotate = pkgErrors.Wrap
	// Annotatef is assigned to "github.com/pkg/errors".Wrapf by default
	Annotatef = pkgErrors.Wrapf
	// Wrap is assigned to "github.com/pkg/errors".Wrap by default
	Wrap = pkgErrors.Wrap
	// Wrapf is assigned to "github.com/pkg/errors".Wrapf by default
	Wrapf = pkgErrors.Wrapf
)
