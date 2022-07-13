# Terraform (TF) Tests

Terraform (TF) tests are an additional form of End-to-End (E2E) tests that cover multi-node K3s configuration and administration: install, update, teardown, etc. across a wide range of operating systems. Terraform tests are used as part of K3s quality assurance (QA) to bring up clusters with different configurations on demand, perform specific functionality tests, and keep them up and running to perform some exploratory tests in real-world scenarios.

## Framework 
TF tests utilize [Ginkgo](https://onsi.github.io/ginkgo/) and [Gomega](https://onsi.github.io/gomega/) like the e2e tests. They rely on [Terraform](https://www.terraform.io/) to provide the underlying cluster configuration. 

## Format

- All TF tests should be placed under `tests/terraform/<TEST_NAME>`.
- All TF test functions should be named: `Test_TF<TEST_NAME>`. 

See the [create cluster test](../tests/terraform/createcluster_test.go) as an example.

## Running

Before running the tests, it's best to create a tfvars file in `./tests/terraform/modules/k3scluster/config/local.tfvars`. There is some information there to get you started, but the empty variables should be filled in appropriately per your AWS environment.

All TF tests can be run with:
```bash
go test -timeout=60m ./tests/terrfaorm/... -run TF
```
Tests can be run individually with:
```bash
go test -timeout=30m ./tests/terraform/createcluster/createcluster.go ./tests/terraform/createcluster/createcluster_test.go
# OR
go test -v -timeout=30m ./tests/terraform/... -run TFClusterCreateValidation
# example with vars:
go test -timeout=30m -v ./tests/terraform/createcluster.go ./tests/terraform/createcluster_test.go -node_os=ubuntu -aws_ami=ami-02f3416038bdb17fb -cluster_type=etcd -resource_name=localrun1 -sshuser=ubuntu -sshkey="key-name" -destroy=false
```

In between tests, if the cluster is not destroyed, then make sure to delete the ./tests/terraform/terraform.tfstate file if you want to create a new cluster.

Additionally, to generate junit reporting for the tests, the Ginkgo CLI is used. Installation instructions can be found [here.](https://onsi.github.io/ginkgo/#getting-started)  

To run the all TF tests and generate JUnit testing reports:
```
ginkgo --junit-report=result.xml ./tests/terraform/...
```

Note: The `go test` default timeout is 10 minutes, thus the `-timeout` flag should be used. The `ginkgo` default timeout is 1 hour, no timeout flag is needed.

# Debugging
The cluster and VMs can be retained after a test by passing `-destroy=false`. 
To focus individual runs on specific test clauses, you can prefix with `F`. For example, in the [create cluster test](../tests/terraform/createcluster_test.go), you can update the initial creation to be: `FIt("Starts up with no issues", func() {` in order to focus the run on only that clause.