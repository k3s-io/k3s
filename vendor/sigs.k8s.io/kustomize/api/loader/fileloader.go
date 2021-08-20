// Copyright 2019 The Kubernetes Authors.
// SPDX-License-Identifier: Apache-2.0

package loader

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"

	"sigs.k8s.io/kustomize/api/ifc"
	"sigs.k8s.io/kustomize/api/internal/git"
	"sigs.k8s.io/kustomize/kyaml/filesys"
)

// fileLoader is a kustomization's interface to files.
//
// The directory in which a kustomization file sits
// is referred to below as the kustomization's _root_.
//
// An instance of fileLoader has an immutable root,
// and offers a `New` method returning a new loader
// with a new root.
//
// A kustomization file refers to two kinds of files:
//
// * supplemental data paths
//
//   `Load` is used to visit these paths.
//
//   These paths refer to resources, patches,
//   data for ConfigMaps and Secrets, etc.
//
//   The loadRestrictor may disallow certain paths
//   or classes of paths.
//
// * bases (other kustomizations)
//
//   `New` is used to load bases.
//
//   A base can be either a remote git repo URL, or
//   a directory specified relative to the current
//   root. In the former case, the repo is locally
//   cloned, and the new loader is rooted on a path
//   in that clone.
//
//   As loaders create new loaders, a root history
//   is established, and used to disallow:
//
//   - A base that is a repository that, in turn,
//     specifies a base repository seen previously
//     in the loading stack (a cycle).
//
//   - An overlay depending on a base positioned at
//     or above it.  I.e. '../foo' is OK, but '.',
//     '..', '../..', etc. are disallowed.  Allowing
//     such a base has no advantages and encourages
//     cycles, particularly if some future change
//     were to introduce globbing to file
//     specifications in the kustomization file.
//
// These restrictions assure that kustomizations
// are self-contained and relocatable, and impose
// some safety when relying on remote kustomizations,
// e.g. a remotely loaded ConfigMap generator specified
// to read from /etc/passwd will fail.
//
type fileLoader struct {
	// Loader that spawned this loader.
	// Used to avoid cycles.
	referrer *fileLoader

	// An absolute, cleaned path to a directory.
	// The Load function will read non-absolute
	// paths relative to this directory.
	root filesys.ConfirmedDir

	// Restricts behavior of Load function.
	loadRestrictor LoadRestrictorFunc

	// If this is non-nil, the files were
	// obtained from the given repository.
	repoSpec *git.RepoSpec

	// File system utilities.
	fSys filesys.FileSystem

	// Used to load from HTTP
	http *http.Client

	// Used to clone repositories.
	cloner git.Cloner

	// Used to clean up, as needed.
	cleaner func() error
}

// NewFileLoaderAtCwd returns a loader that loads from PWD.
// A convenience for kustomize edit commands.
func NewFileLoaderAtCwd(fSys filesys.FileSystem) *fileLoader {
	return newLoaderOrDie(
		RestrictionRootOnly, fSys, filesys.SelfDir)
}

// NewFileLoaderAtRoot returns a loader that loads from "/".
// A convenience for tests.
func NewFileLoaderAtRoot(fSys filesys.FileSystem) *fileLoader {
	return newLoaderOrDie(
		RestrictionRootOnly, fSys, filesys.Separator)
}

// Root returns the absolute path that is prepended to any
// relative paths used in Load.
func (fl *fileLoader) Root() string {
	return fl.root.String()
}

func newLoaderOrDie(
	lr LoadRestrictorFunc,
	fSys filesys.FileSystem, path string) *fileLoader {
	root, err := demandDirectoryRoot(fSys, path)
	if err != nil {
		log.Fatalf("unable to make loader at '%s'; %v", path, err)
	}
	return newLoaderAtConfirmedDir(
		lr, root, fSys, nil, git.ClonerUsingGitExec)
}

// newLoaderAtConfirmedDir returns a new fileLoader with given root.
func newLoaderAtConfirmedDir(
	lr LoadRestrictorFunc,
	root filesys.ConfirmedDir, fSys filesys.FileSystem,
	referrer *fileLoader, cloner git.Cloner) *fileLoader {
	return &fileLoader{
		loadRestrictor: lr,
		root:           root,
		referrer:       referrer,
		fSys:           fSys,
		cloner:         cloner,
		cleaner:        func() error { return nil },
	}
}

// Assure that the given path is in fact a directory.
func demandDirectoryRoot(
	fSys filesys.FileSystem, path string) (filesys.ConfirmedDir, error) {
	if path == "" {
		return "", fmt.Errorf(
			"loader root cannot be empty")
	}
	d, f, err := fSys.CleanedAbs(path)
	if err != nil {
		return "", err
	}
	if f != "" {
		return "", fmt.Errorf(
			"got file '%s', but '%s' must be a directory to be a root",
			f, path)
	}
	return d, nil
}

// New returns a new Loader, rooted relative to current loader,
// or rooted in a temp directory holding a git repo clone.
func (fl *fileLoader) New(path string) (ifc.Loader, error) {
	if path == "" {
		return nil, fmt.Errorf("new root cannot be empty")
	}

	repoSpec, err := git.NewRepoSpecFromUrl(path)
	if err == nil {
		// Treat this as git repo clone request.
		if err = fl.errIfRepoCycle(repoSpec); err != nil {
			return nil, err
		}
		return newLoaderAtGitClone(
			repoSpec, fl.fSys, fl, fl.cloner)
	}

	if filepath.IsAbs(path) {
		return nil, fmt.Errorf("new root '%s' cannot be absolute", path)
	}
	root, err := demandDirectoryRoot(fl.fSys, fl.root.Join(path))
	if err != nil {
		return nil, err
	}
	if err = fl.errIfGitContainmentViolation(root); err != nil {
		return nil, err
	}
	if err = fl.errIfArgEqualOrHigher(root); err != nil {
		return nil, err
	}
	return newLoaderAtConfirmedDir(
		fl.loadRestrictor, root, fl.fSys, fl, fl.cloner), nil
}

// newLoaderAtGitClone returns a new Loader pinned to a temporary
// directory holding a cloned git repo.
func newLoaderAtGitClone(
	repoSpec *git.RepoSpec, fSys filesys.FileSystem,
	referrer *fileLoader, cloner git.Cloner) (ifc.Loader, error) {
	cleaner := repoSpec.Cleaner(fSys)
	err := cloner(repoSpec)
	if err != nil {
		cleaner()
		return nil, err
	}
	root, f, err := fSys.CleanedAbs(repoSpec.AbsPath())
	if err != nil {
		cleaner()
		return nil, err
	}
	// We don't know that the path requested in repoSpec
	// is a directory until we actually clone it and look
	// inside.  That just happened, hence the error check
	// is here.
	if f != "" {
		cleaner()
		return nil, fmt.Errorf(
			"'%s' refers to file '%s'; expecting directory",
			repoSpec.AbsPath(), f)
	}
	return &fileLoader{
		// Clones never allowed to escape root.
		loadRestrictor: RestrictionRootOnly,
		root:           root,
		referrer:       referrer,
		repoSpec:       repoSpec,
		fSys:           fSys,
		cloner:         cloner,
		cleaner:        cleaner,
	}, nil
}

func (fl *fileLoader) errIfGitContainmentViolation(
	base filesys.ConfirmedDir) error {
	containingRepo := fl.containingRepo()
	if containingRepo == nil {
		return nil
	}
	if !base.HasPrefix(containingRepo.CloneDir()) {
		return fmt.Errorf(
			"security; bases in kustomizations found in "+
				"cloned git repos must be within the repo, "+
				"but base '%s' is outside '%s'",
			base, containingRepo.CloneDir())
	}
	return nil
}

// Looks back through referrers for a git repo, returning nil
// if none found.
func (fl *fileLoader) containingRepo() *git.RepoSpec {
	if fl.repoSpec != nil {
		return fl.repoSpec
	}
	if fl.referrer == nil {
		return nil
	}
	return fl.referrer.containingRepo()
}

// errIfArgEqualOrHigher tests whether the argument,
// is equal to or above the root of any ancestor.
func (fl *fileLoader) errIfArgEqualOrHigher(
	candidateRoot filesys.ConfirmedDir) error {
	if fl.root.HasPrefix(candidateRoot) {
		return fmt.Errorf(
			"cycle detected: candidate root '%s' contains visited root '%s'",
			candidateRoot, fl.root)
	}
	if fl.referrer == nil {
		return nil
	}
	return fl.referrer.errIfArgEqualOrHigher(candidateRoot)
}

// TODO(monopole): Distinguish branches?
// I.e. Allow a distinction between git URI with
// path foo and tag bar and a git URI with the same
// path but a different tag?
func (fl *fileLoader) errIfRepoCycle(newRepoSpec *git.RepoSpec) error {
	// TODO(monopole): Use parsed data instead of Raw().
	if fl.repoSpec != nil &&
		strings.HasPrefix(fl.repoSpec.Raw(), newRepoSpec.Raw()) {
		return fmt.Errorf(
			"cycle detected: URI '%s' referenced by previous URI '%s'",
			newRepoSpec.Raw(), fl.repoSpec.Raw())
	}
	if fl.referrer == nil {
		return nil
	}
	return fl.referrer.errIfRepoCycle(newRepoSpec)
}

// Load returns the content of file at the given path,
// else an error.  Relative paths are taken relative
// to the root.
func (fl *fileLoader) Load(path string) ([]byte, error) {
	if u, err := url.Parse(path); err == nil && (u.Scheme == "http" || u.Scheme == "https") {
		var hc *http.Client
		if fl.http != nil {
			hc = fl.http
		} else {
			hc = &http.Client{}
		}
		resp, err := hc.Get(path)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}
		return body, nil
	}
	if !filepath.IsAbs(path) {
		path = fl.root.Join(path)
	}
	path, err := fl.loadRestrictor(fl.fSys, fl.root, path)
	if err != nil {
		return nil, err
	}
	return fl.fSys.ReadFile(path)
}

// Cleanup runs the cleaner.
func (fl *fileLoader) Cleanup() error {
	return fl.cleaner()
}
