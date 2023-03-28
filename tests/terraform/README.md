# Terraform (TF) Tests

Terraform (TF) tests are an additional form of End-to-End (E2E) tests that cover multi-node K3s configuration and administration: install, update, teardown, etc. across a wide range of operating systems. Terraform tests are used as part of K3s quality assurance (QA) to bring up clusters with different configurations on demand, perform specific functionality tests, and keep them up and running to perform some exploratory tests in real-world scenarios.

## Framework 
TF tests utilize [Ginkgo](https://onsi.github.io/ginkgo/) and [Gomega](https://onsi.github.io/gomega/) like the e2e tests. They rely on [Terraform](https://www.terraform.io/) to provide the underlying cluster configuration. 

## Format

- All TF tests should be placed under `tests/terraform/<TEST_NAME>`.
- All TF test functions should be named: `Test_TF<TEST_NAME>`. 

See the [create cluster test](../tests/terraform/createcluster_test.go) as an example.

## Running

- Before running the tests, you should creat local.tfvars file in `./tests/terraform/modules/k3scluster/config/local.tfvars`. There is some information there to get you started, but the empty variables should be filled in appropriately per your AWS environment.



- For running tests with "etcd" cluster type, you should add the value "etcd" to the variable "cluster_type" , also you need have those variables at least empty:
```
- external_db       
- external_db_version
- instance_class  
- db_group_name
```

- For running with external db you need the same variables above filled in with the correct data and also cluster_type= ""


All TF tests can be run with:
```bash
go test -timeout=60m ./tests/terraform/... -run TF
```
Tests can be run individually with:
```bash
go test -timeout=30m ./tests/terraform/createcluster/createcluster.go ./tests/terraform/createcluster/createcluster_test.go
# OR
go test -v -timeout=30m ./tests/terraform/... -run TFClusterCreateValidation
# example with vars:
go test -timeout=30m -v ./tests/terraform/createcluster.go ./tests/terraform/createcluster_test.go -node_os=ubuntu -aws_ami=ami-02f3416038bdb17fb -cluster_type=etcd -resource_name=localrun1 -sshuser=ubuntu -sshkey="key-name" -destroy=false

```
Test Flags:
```
- ${upgradeVersion} version to upgrade to
```
We can also run tests through the Makefile through tests' directory:

- On the first run with make and docker please delete your .terraform folder, terraform.tfstate and terraform.hcl.lock file

```bash
Args:
*All args are optional and can be used with:

`$make tf-run`         `$make tf-logs`,
`$make vet-lint`       `$make tf-complete`, 
`$make tf-upgrade`     `$make tf-test-suite-same-cluster`,
`$make tf-test-suite`


- ${IMGNAME}     append any string to the end of image name
- ${TAGNAME}     append any string to the end of tag name
- ${ARGNAME}     name of the arg to pass to the test
- ${ARGVALUE}    value of the arg to pass to the test
- ${TESTDIR}     path to the test directory 

Commands:
$ make tf-up                         # create the image from Dockerfile.build
$ make tf-run                        # runs all tests if no flags or args provided
$ make tf-down                       # removes the image
$ make tf-clean                      # removes instances and resources created by tests
$ make tf-logs                       # prints logs from container the tests
$ make tf-complete                   # clean resources + remove images + run tests
$ make tf-create                     # runs create cluster test locally
$ make tf-upgrade                    # runs upgrade cluster test locally
$ make tf-test-suite-same-cluster    # runs all tests locally in sequence using the same state    
$ make tf-remove-state               # removes terraform state dir and files
$ make tf-test-suite                 # runs all tests locally in sequence not using the same state
$ make vet-lint                      # runs go vet and go lint

      
Examples:
$ make tf-up TAGNAME=ubuntu
$ make tf-run IMGNAME=2 TAGNAME=ubuntu TESTDIR=upgradecluster ARGNAME=upgradeVersion ARGVALUE=v1.26.2+k3s1
$ make tf-run TESTDIR=upgradecluster
$ make tf-logs IMGNAME=1
$ make vet-lint TESTDIR=upgradecluster
```


# Running tests in parallel:
- You can play around and have a lot of different test combinations like:
```
- Build docker image with different TAGNAME="OS`s" + with different configurations( resource_name, node_os, versions, install type, nodes and etc) and have unique "IMGNAMES"

- And in the meanwhile run also locally with different configuration while your dockers TAGNAME and IMGNAMES are running
```

# In between tests:
- If you want to run with same cluster do not delete ./tests/terraform/modules/terraform.tfstate + .terraform.lock.hcl file after each test.

- if you want to use new resources then make sure to delete the ./tests/terraform/modules/terraform.tfstate + .terraform.lock.hcl file if you want to create a new cluster.


# Common Issues:

- Issues related to terraform plugin please also delete the modules/.terraform folder
- In mac m1 maybe you need also to go to rke2/tests/terraform/modules and run `terraform init` to download the plugins




# Reporting:
Additionally, to generate junit reporting for the tests, the Ginkgo CLI is used. Installation instructions can be found [here.](https://onsi.github.io/ginkgo/#getting-started)  

To run the all TF tests and generate JUnit testing reports:
```
ginkgo --junit-report=result.xml ./tests/terraform/...
```

Note: The `go test` default timeout is 10 minutes, thus the `-timeout` flag should be used. The `ginkgo` default timeout is 1 hour, no timeout flag is needed.

# Debugging
The cluster and VMs can be retained after a test by passing `-destroy=false`. 
To focus individual runs on specific test clauses, you can prefix with `F`. For example, in the [create cluster test](../tests/terraform/createcluster_test.go), you can update the initial creation to be: `FIt("Starts up with no issues", func() {` in order to focus the run on only that clause.