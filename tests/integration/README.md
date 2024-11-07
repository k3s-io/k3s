# Integration Tests

Integration tests should be used to test a specific functionality of k3s that exists across multiple Go packages, either via exported function calls, or more often, CLI commands.
Integration tests should be used for "black box" testing. 

## Framework

All integration tests in K3s follow a [Behavior Diven Development (BDD)](https://en.wikipedia.org/wiki/Behavior-driven_development) style. Specifically, K3s uses [Ginkgo](https://onsi.github.io/ginkgo/) and [Gomega](https://onsi.github.io/gomega/) to drive the tests.  
To generate an initial test, the command `ginkgo bootstrap` can be used.

To facilitate K3s CLI testing, see `tests/util/cmd.go` helper functions.

## Format

All integration tests should be placed under `tests/integration/<TEST_NAME>`.  
All integration test files should be named: `<TEST_NAME>_int_test.go`.  
All integration test functions should be named: `Test_Integration<TEST_NAME>`.  
See the [local storage test](../tests/integration/localstorage/localstorage_int_test.go) as an example.

## Running

Integration tests can be run with no k3s cluster present, each test will spin up and kill the appropriate k3s server it needs.  
Note: Integration tests must be run as root, prefix the commands below with `sudo -E env "PATH=$PATH"` if a sudo user.
```bash
go test ./tests/integration/... -run Integration -ginkgo.v -test.v
```

Additionally, to generate JUnit reporting for the tests, the Ginkgo CLI is used
```
ginkgo --junit-report=result.xml ./tests/integration/...
```

Integration tests can be run on an existing single-node cluster via compile time flag, tests will skip if the server is not configured correctly.
```bash
go test -ldflags "-X 'github.com/k3s-io/k3s/tests/integration.existingServer=True'" ./tests/integration/... -run Integration -ginkgo.v -test.v
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
