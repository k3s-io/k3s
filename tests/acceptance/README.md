## Acceptance Framework

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
│   └───── Entry for tests execution, separated by test runs and test suites
│
├── modules
│   └───── Terraform modules and configurations
│
├── shared
    └───── shared and reusable functions, workloads, constants, and scripts

```

### Explanation:

- `Entrypoint`
````
Act:                  Acts as the one of the outter layer to receive the input to start test execution
Responsibility:       Should not implement any logic and only focus on orchestrating that
````

- `Core`
```
    Service:
  
Act:                  Acts as a provider for customizations through ser across framework
Responsibility:       Should not depend on any outer layer only in the core itself, provide services rather than rely on.
 
  
    Testcase:
  
Act:                  Acts as an innermost layer where the main logic (test implementation) is handled.
Responsibility:       Encapsulates test logic and should not depend on any outer layer
```

- `Modules`
```
Act:                  Acts as the infra to provide the terraform modules and configurations
Responsibility:       Only provides indirectly for all, should not need the knowledge of any test logic or have dependencies from internal layers.
``` 

- `Shared`
```
Act:                  Acts as an intermediate module providing shared and reusable functions, workloads, constants, and scripts               
Responsibility:       Should not need knowledge of or "external" dependencies at all and provides for all layers.
```


#### PS: "External" and "Outer" layer or dependency here in this context is considered any other package within the framework.

-------------------

#### Testcase naming convention:
- All tests should be placed under `tests/acceptance/testcase/<TESTNAME>`.
- All test functions should be named: `Test<TESTNAME>`.


## Running

- Before running the tests, you should creat local.tfvars file in `./tests/acceptance/modules/k3scluster/config/local.tfvars`. There is some information there to get you started, but the empty variables should be filled in appropriately per your AWS environment.


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

###  Run with `Makefile` through acceptance package:
```bash
- On the first run with make and docker please delete your .terraform folder, terraform.tfstate and terraform.hcl.lock file

Args:
*Most of args are optional so you can fit to your use case.

- ${IMGNAME}               append any string to the end of image name
- ${TAGNAME}               append any string to the end of tag name
- ${ARGNAME}               name of the arg to pass to the test
- ${ARGVALUE}              value of the arg to pass to the test
- ${TESTDIR}               path to the test directory 
- ${TESTFILE}              path to the test file
- ${TAGTEST}               name of the tag function from suite ( -tags=upgradesuc or -tags=upgrademanual )
- ${TESTCASE}              name of the testcase to run
- ${DEPLOYWORKLOAD}        true or false to deploy workload
- ${CMDHOST}               command to run on host
- ${VALUEHOST}             value to check on host
- ${VALUEHOSTUPGRADED}     value to check on host after upgrade
- ${CMDNODE}               command to run on node
- ${VALUENODE}             value to check on node
- ${VALUENODEUPGRADED}     value to check on node after upgrade
- ${INSTALLTYPE}           type of installation (version or commit) + desired value


Commands: 
$ make test-env-up                     # create the image from Dockerfile.build
$ make test-run                        # runs create and upgrade cluster by passing the argname and argvalue
$ make test-env-down                   # removes the image and container by prefix
$ make test-env-clean                  # removes instances and resources created by testcase
$ make test-logs                       # prints logs from container the testcase
$ make test-complete                   # clean resources + remove images + run testcase
$ make test-create                     # runs create cluster test locally
$ make test-upgrade                    # runs upgrade cluster test locally
$ make test-version-local-path         # runs version bump for local path storage test locally
$ make test-version-bump               # runs version bump test locally
$ make test-run                        # runs create and upgrade cluster by passing the argname and argvalue
$ make remove-tf-state                 # removes acceptance state dir and files
$ make test-suite                      # runs all testcase locally in sequence not using the same state
$ make vet-lint                        # runs go vet and go lint

      
Examples: 

- Create an image tagged
$ make test-env-up TAGNAME=ubuntu


- Run upgrade cluster test with ${IMGNAME} and ${TAGNAME}
$ make test-run IMGNAME=2 TAGNAME=ubuntu TESTDIR=upgradecluster ARGNAME=upgradeVersion ARGVALUE=v1.26.2+k3s1


- Run create and upgrade cluster just adding ARGNAME and ARGVALUE flag to upgrade
$ make tf-run ARGNAME=upgradeVersion ARGVALUE=v1.26.2+k3s1


- Run version bump test upgrading with commit id
$ make test-run IMGNAME=x \
TAGNAME=y \
TESTDIR=versionbump \
CMDNODE="k3s --version" \
VALUENODE="v1.26.2+k3s1" \
CMDHOST="kubectl get image..."  \
VALUEHOST="v0.0.21" \
INSTALLTYPE=INSTALL_K3S_COMMIT=257fa2c54cda332e42b8aae248c152f4d1898218 \ 
TESTCASE=TestLocalPathProvisionerStorage \
DEPLOYWORKLOAD=true


- Run bump version local path provisioner upgrading with version
$ make test-run IMGNAME=23 \
TAGNAME=1 \
TESTDIR=versionbump \
TESTTAG=localpath \
VALUENODE=v1.26.2+k3s1 \
VALUENODEUPGRADED=v1.27.1-rc1+k3s1 \
VALUEHOST=v0.0.21 \
VALUEHOSTUPGRADED=v0.0.22 \
INSTALLTYPE=INSTALL_K3S_VERSION=v1.27.1-rc1+k3s1 \


- Logs from test
$ make tf-logs IMGNAME=1

- Run lint for a specific directory
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