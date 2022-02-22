# Code conventions

- [POSIX shell](#posix-shell)
- [Go](#go)
- [Directory and file conventions](#directory-and-file-conventions)
- [Testing conventions](#testing-conventions)

## POSIX shell

- [Style guide](https://google.github.io/styleguide/shell.xml)

## Go

- [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)
- [Effective Go](https://golang.org/doc/effective_go.html)
- Know and avoid [Go landmines](https://gist.github.com/lavalamp/4bd23295a9f32706a48f)
- Comment your code.
  - [Go's commenting conventions](http://blog.golang.org/godoc-documenting-go-code)
  - If reviewers ask questions about why the code is the way it is, that's a sign that comments might be helpful.
- Command-line flags should use dashes, not underscores
- Naming
  - Please consider package name when selecting an interface name, and avoid redundancy. For example, `storage.Interface` is better than `storage.StorageInterface`.
  - Do not use uppercase characters, underscores, or dashes in package names.
  - Please consider parent directory name when choosing a package name. For example, `pkg/controllers/autoscaler/foo.go` should say `package autoscaler` not `package autoscalercontroller`.
    - Unless there's a good reason, the `package foo` line should match the name of the directory in which the `.go` file exists.
    - Importers can use a different name if they need to disambiguate.

## Directory and file conventions

- Avoid general utility packages. Packages called "util" are suspect. Instead, derive a name that describes your desired function. For example, the utility functions dealing with waiting for operations are in the `wait` package and include functionality like `Poll`. The full name is `wait.Poll`.
- All filenames should be lowercase.
- All source files and directories should use underscores, not dashes.
  - Package directories should generally avoid using separators as much as possible. When package names are multiple words, they usually should be in nested subdirectories.

## Testing conventions

Please refer to [TESTING.md](../../tests/TESTING.md) document.
