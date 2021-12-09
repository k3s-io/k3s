Contributing
============

The `wgctrl` project makes use of the [GitHub Flow](https://guides.github.com/introduction/flow/)
for contributions.

If you'd like to contribute to the project, please
[open an issue](https://github.com/WireGuard/wgctrl-go/issues/new) or find an
[existing issue](https://github.com/WireGuard/wgctrl-go/issues) that you'd like
to take on.  This ensures that efforts are not duplicated, and that a new feature
aligns with the focus of the rest of the repository.

Once your suggestion has been submitted and discussed, please be sure that your
code meets the following criteria:

- code is completely `gofmt`'d
- new features or codepaths have appropriate test coverage
- `go test ./...` passes
- `go vet ./...` passes
- `staticcheck ./...` passes
- `golint ./...` returns no warnings, including documentation comment warnings

Finally, submit a pull request for review!
