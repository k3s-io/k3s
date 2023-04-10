## k3sTF Framework

- It relies on [Terraform](https://www.terraform.io/) to provide the underlying cluster configuration.
- It uses [Ginkgo](https://onsi.github.io/ginkgo/) and [Gomega](https://onsi.github.io/gomega/) as assertion framework.

## Architecture
- For better maintenance, readability and productivity we encourage max of separation of concerns and loose coupling between packages so inner packages should not depend on outer packages

### Packages:
```bash
./acceptance
│
├── core
│   └───── Place where resides the logic and services for it
│
├── entrypoint
│   └───── Where is located the entrypoint for tests execution, separated by test runs and test suites
│
├── modules
│   └───── Terraform modules and configurations
│
├── shared
│   └───── shared and reusable util functions, workloads and scripts

```

### Explanation:

- `Entrypoint`
````
Act:                  it acts as the highest or outter layer to receive the input to start and manage tests execution
Responsibility:       should not need to implement any logic 
````

- `Core`
```
    Service:
  
Act:                  it acts as a provider for customizations through ser across framework
Responsibility:       should not depend on any outer layer only in the core itself, the idea is to provide not rely on.
 
  
    Testcase:
  
Act:                  it acts as a innermost layer where the main logic (test implementations) is handled.
Responsibility:       encapsulate test logic to, should not depend on any outer layer
```

- `Modules`
```
Act:                  it acts as the infra to provide the terraform modules and configurations
Responsibility:       Only provides indirectly for all, should not need the knowledge of any test logic or have dependencies from internal layers.
``` 

- `Shared`
```
Act:                  it acts as an intermediate module providing shared and reusable workloads and scripts               
Responsibility:       should not need the knowledge or "external" dependency at all, provides for all.
```


#### PS: "External" and "Outer" layer or dependency here in this context is considered any other package within the framework.

-------------------

#### Testcase naming convention:
- All tests should be placed under `tests/acceptance/testcase/<TESTNAME>`.
- All test functions should be named: `Test<TESTNAME>`.


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


Tests can be run individually per package:
```bash
go test -timeout=45m -v ./tests/acceptance/entrypoint/$PACKAGE_NAME/...
```
Test flags:
```
 ${upgradeVersion} version to upgrade to
    -upgradeVersion=v1.26.2+k3s1
    
 ${installType} type of installation (version or commit) + desired value    
    -installType=version or commit
```

###  Run with `Makefile` through k3sTF package:
```bash
- On the first run with make and docker please delete your .terraform folder, terraform.tfstate and terraform.hcl.lock file

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
- ${TESTFILE}    path to the test file
- ${TAGTEST}     name of the tag function from suite ( -tags=upgradesuc or -tags=upgrademanual )

Commands:
$ make tf-up                         # create the image from Dockerfile.build
$ make tf-run                        # runs all testcase if no flags or args provided
$ make tf-down                       # removes the image
$ make tf-clean                      # removes instances and resources created by testcase
$ make tf-logs                       # prints logs from container the testcase
$ make tf-complete                   # clean resources + remove images + run testcase
$ make tf-create                     # runs create cluster test locally
$ make tf-upgrade                    # runs upgrade cluster test locally
$ make tf-test-suite-same-cluster    # runs all testcase locally in sequence using the same state    
$ make tf-remove-state               # removes acceptance state dir and files
$ make tf-test-suite                 # runs all testcase locally in sequence not using the same state
$ make vet-lint                      # runs go vet and go lint

      
Examples:
$ make tf-up TAGNAME=ubuntu
$ make tf-run IMGNAME=2 TAGNAME=ubuntu TESTDIR=upgradecluster ARGNAME=upgradeVersion ARGVALUE=v1.26.2+k3s1
$ make tf-run TESTDIR=upgradecluster
$ make tf-logs IMGNAME=1
$ make vet-lint TESTDIR=upgradecluster
```


### Running tests in parallel:
- You can play around and have a lot of different test combinations like:
```
- Build docker image with different TAGNAME="OS`s" + with different configurations( resource_name, node_os, versions, install type, nodes and etc) and have unique "IMGNAMES"

- And in the meanwhile run also locally with different configuration while your dockers TAGNAME and IMGNAMES are running
```

### In between tests:
```
- If you want to run with same cluster do not delete ./tests/terraform/modules/terraform.tfstate + .terraform.lock.hcl file after each test.

- if you want to use new resources then make sure to delete the ./tests/terraform/modules/terraform.tfstate + .terraform.lock.hcl file if you want to create a new cluster.
```

###  Common Issues:
````
- Issues related to terraform plugin please also delete the modules/.terraform folder
- In mac m1 maybe you need also to go to rke2/tests/terraform/modules and run `terraform init` to download the plugins
````



### Reporting:
```
Additionally, to generate junit reporting for the tests, the Ginkgo CLI is used. Installation instructions can be found [here.](https://onsi.github.io/ginkgo/#getting-started)  

To run the all TF tests and generate JUnit testing reports:

ginkgo --junit-report=result.xml ./tests/terraform/cases/...


Note: The `go test` default timeout is 10 minutes, thus the `-timeout` flag should be used. The `ginkgo` default timeout is 1 hour, no timeout flag is needed
````
### Debugging
````
The cluster and VMs can be retained after a test by passing `-destroy=false`. 
To focus individual runs on specific test clauses, you can prefix with `F`. For example, in the [create cluster test](../tests/terraform/cases/createcluster_test.go), you can update the initial creation to be: `FIt("Starts up with no issues", func() {` in order to focus the run on only that clause.