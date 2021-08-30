# Testing Standards in K3s

Go testing in K3s comes in 3 forms: Unit, Integration, and End-to-End (E2E). This
document will explain *when* each test should be written and *how* each test should be
generated, formatted, and run.

Note: all shell commands given are relateive to the root k3s repo directory.
___

## Unit Tests

Unit tests should be written when a component or function of a package needs testing.
Unit tests should be used for "white box" testing.

### Framework

All unit tests in K3s follow a [Table Driven Test](https://github.com/golang/go/wiki/TableDrivenTests) style. Specifically, K3s unit tests are automatically generated using the [gotests](https://github.com/cweill/gotests) tool. This is built into the Go vscode extension, has documented integrations for other popular editors, or can be run via command line. Additionally, a set of custom templates are provided to extend the generated test's functionality. To use these templates, call:

```bash
gotests --template_dir=<PATH_TO_K3S>/contrib/gotests_templates
```

Or in vscode, edit the Go extension setting `Go: Generate Tests Flags`  
and add `--template_dir=<PATH_TO_K3S>/contrib/gotests_templates` as an item.

To facilitate unit test creation, see `tests/util/runtime.go` helper functions.

### Format

All unit tests should be placed within the package of the file they test.  
All unit test files should be named: `<FILE_UNDER_TEST>_test.go`.  
All unit test functions should be named: `Test_Unit<FUNCTION_TO_TEST>` or `Test_Unit<RECEIVER>_<METHOD_TO_TEST>`.  
See the [etcd unit test](https://github.com/k3s-io/k3s/blob/master/pkg/etcd/etcd_test.go) as an example.

### Running

```bash
go test ./pkg/... -run Unit
```

Note: As unit tests call functions directly, they are the primary drivers of K3s's code coverage
metric.

___

## Integration Tests

Integration tests should be used to test a specific functionality of k3s that exists across multiple Go packages, either via exported function calls, or more often, CLI comands.
Integration tests should be used for "black box" testing. 

### Framework

All integration tests in K3s follow a [Behavior Diven Development (BDD)](https://en.wikipedia.org/wiki/Behavior-driven_development) style. Specifically, K3s uses [Ginkgo](https://onsi.github.io/ginkgo/) and [Gomega](https://onsi.github.io/gomega/) to drive the tests.  
To generate an initial test, the command `ginkgo bootstrap` can be used.

To facilitate K3s CLI testing, see `tests/util/cmd.go` helper functions.

### Format

Integration tests can be placed in two areas:  

1. Next to the go package they intend to test.
2. In `tests/integration/` for package agnostic testing.  

Package specific integration tests should use the `<PACKAGE_UNDER_TEST>_test` package.  
Package agnostic integration tests should use the `integration` package.  
All integration test files should be named: `<TEST_NAME>_int_test.go`  
All integration test functions should be named: `Test_Integration<Test_Name>`.  
See the [etcd snapshot test](https://github.com/k3s-io/k3s/blob/master/pkg/etcd/etcd_int_test.go) as a package specific example.  
See the [local storage test](https://github.com/k3s-io/k3s/blob/master/tests/integration/localstorage_int_test.go) as a package agnostic example.

### Running

Integration tests can be run with no k3s cluster present, each test will spin up and kill the appropriate k3s server it needs.
```bash
go test ./pkg/... ./tests/... -run Integration
```

Integration tests can be run on an existing single-node cluster via compile time flag, tests will skip if the server is not configured correctly.
```bash
go test -ldflags "-X 'github.com/rancher/k3s/tests/util.existingServer=True'" ./pkg/... ./tests/... -run Integration
```

Integration tests can also be run via a [Sonobuoy](https://sonobuoy.io/docs/v0.53.2/) plugin on an existing single-node cluster.
```bash
./scripts/build-tests-sonobuoy
sudo KUBECONFIG=/etc/rancher/k3s/k3s.yaml sonobuoy run --plugin ./dist/artifacts/k3s-int-tests.yaml
```
Check the sonobuoy status and retrieve results
``` 
sudo KUBECONFIG=/etc/rancher/k3s/k3s.yaml sonobuoy status
sudo KUBECONFIG=/etc/rancher/k3s/k3s.yaml sonobuoy retrieve
sudo KUBECONFIG=/etc/rancher/k3s/k3s.yaml sonobuoy results <TAR_FILE_FROM_RETRIEVE>
```

___

## End-to-End (E2E) Tests

End-to-end tests utilize [Ginkgo](https://onsi.github.io/ginkgo/) and [Gomega](https://onsi.github.io/gomega/) like the integration tests, but rely on separate testing utilities and are self-contained within the `test/e2e` directory. E2E tests cover complete K3s single and multi-cluster configuration and administration: bringup, update, teardown etc.  
E2E tests are run nightly as part of K3s quality assurance (QA).

### Format

All E2E tests should be placed under the `e2e` package.  
All E2E test functions should be named: `Test_E2E<TEST_NAME>`.  
See the [upgrade cluster test](https://github.com/k3s-io/k3s/blob/master/tests/e2e/upgradecluster_test.go) as an example.

### Running

Generally, E2E tests are run as a nightly Jenkins job for QA. They can still be run locally but additional setup may be required.

```bash
go test ./tests/... -run E2E
```

## Contributing New Or Updated Tests

___
We gladly accept new and updated tests of all types. If you wish to create
a new test or update an existing test, please submit a PR with a title that includes the words `<NAME_OF_TEST> (Created/Updated)`.
