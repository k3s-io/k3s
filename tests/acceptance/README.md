## Acceptance Test Framework

The acceptance tests are a customizable way to create clusters and perform validations on them such that the requirements of specific features and functions can be validated.

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
│── scripts
│    └───── Scripts needed for overall execution
│
├── shared
│    └───── auxiliary and reusable functions
│
│── workloads
│   └───── Place where resides workloads to use inside tests
```

### Explanation:

- `Core`
```
    Service:
  
Act:                  Acts as a provider for customizations across framework
Responsibility:       Should not depend on any outer layer only in the core itself, provide services rather than rely on.
 
  
    Testcase:
  
Act:                  Acts as an innermost layer where the main logic (test implementation) is handled.
Responsibility:       Encapsulates test logic and should not depend on any outer layer
```

- `Entrypoint`
````
Act:                  Acts as the one of the outer layer to receive the input to start test execution
Responsibility:       Should not implement any logic and only focus on orchestrating
````

- `Modules`
```
Act:                  Acts as the infra to provide the terraform modules and configurations
Responsibility:       Only provides indirectly for all, should not need the knowledge of any test logic or have dependencies from internal layers.
```

- `Scripts`
```
Act:                  Acts as a provider for scripts needed for overall execution
Responsibility:       Should not need knowledge of or "external" dependencies at all and provides for all layers.
```

- `Shared`
```
Act:                  Acts as an intermediate module providing shared, reusable and auxiliary functions
Responsibility:       Should not need knowledge of or "external" dependencies at all and provides for all layers.
```

- `Workloads`
````
Act:                  Acts as a provider for test workloads
Responsibility:       Totally independent of any other layer and should only provide
````

#### PS: "External" and "Outer" layer or dependency here in this context is considered any other package within the framework.

-------------------


### `Template Bump Version Model`

- We have a template model interface for testing bump versions, the idea is to provide a simple and direct way to test bump of version using go test tool.


```You can test that like:```

- Adding one version or commit and ran some commands on it and check it against respective expected values then upgrade and repeat the same commands and check the respective new (or not) expected values.



```How can I do that?```

- Step 1: Add your desired first version or commit that you want to use on `local.tfvars` file on the vars `k3s_version` and `install_mode`
- Step 2: Have the commands you need to run and the expected output from them
- Step 3: Have a version or commit that you want to upgrade to.
- Step 4: On the TestConfig field you can add another test case that we already have or a newly created one.
- Step 5: You can add a standalone workload deploy if you need
- Step 6: Just fill the go test or make command with your required values
- Step 7: Run the command and wait for results.
- Step 8: (WIP) Export your customizable report.

-------------------

Available arguments to create your command with examples:
````
- $ -cmd "kubectl describe pod -n kube-system local-path-provisioner- : | grep -i Image"
- $ -expectedValue "v0.0.21"
- $ -expectedValueUpgrade "v0.0.24"
- $ -installVersionOrCommit INSTALL_K3S_COMMIT=257fa2c54cda332e42b8aae248c152f4d1898218
- $ -deployWorkload true
- $ -testCase "TestLocalPathProvisionerStorage"
- $ -workloadName "bandwidth-annotations.yaml"
- $ -description "Description of your test"

* All non-boolean arguments is comma separated in case you need to send more than 1.

* If you need to separate another command to run as a single here , separate those with " : " as this example:
`-cmd "kubectl describe pod -n kube-system local-path-provisioner- :  | grep -i Image"`
````

Example of an execution with multiple values:
```bash
go test -timeout=45m -v -tags=versionbump  ./entrypoint/versionbump/... \
-cmd "/var/lib/rancher/k3s/data/current/bin/cni, kubectl get pod test-pod -o yaml : | grep -A2 annotations, k3s -v" \
-expectedValue "v1.2.0-k3s1,1M, v1.26" \
-expectedValueUpgrade "v1.2.0-k3s1,1M, v1.27" \
-installVersionOrCommit INSTALL_K3S_VERSION=v1.27.2+k3s1 \
-testCase "TestServiceClusterIp, TestLocalPathProvisionerStorage" \
-deployWorkload=true \
-workloadName "bandwidth-annotations.yaml"
````


Example of an execution with less args:
````bash
go test -timeout=45m -v -tags=versionbump  ./entrypoint/versionbump/... \
-cmd "/var/lib/rancher/k3s/data/current/bin/cni, kubectl get pod test-pod -o yaml : | grep -A2 annotations, k3s -v"  \
-expectedValue "v1.2.0-k3s1,1M, v1.26"  \
-expectedValueUpgrade "v1.2.0-k3s1,1M, v1.27" \
-installVersionOrCommit INSTALL_K3S_VERSION=v1.27.2+k3s1 \
````


#### We also have this on the `makefile` to make things easier to run just adding the values, please see bellow on the makefile section


-----
#### Testcase naming convention:
- All tests should be placed under `tests/acceptance/testcase/<TESTNAME>`.
- All test functions should be named: `Test<TESTNAME>`.


## Running

- Before running the tests, you should creat local.tfvars file in `./tests/acceptance/modules/k3scluster/config/local.tfvars`. There is some information there to get you started, but the empty variables should be filled in appropriately per your AWS environment.

- Please make sure to export your correct AWS credentials before running the tests. e.g:
```bash
export AWS_ACCESS_KEY_ID=<YOUR_AWS_ACCESS_KEY_ID>
export AWS_SECRET_ACCESS_KEY=<YOUR_AWS_SECRET_ACCESS_KEY>
```

- The local.tfvars split roles section should be strictly followed to not cause any false positives or negatives on tests

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
 ${installVersionOrCommit} type of installation (version or commit) + desired value
    -installVersionOrCommit version or commit

```

Test tags:
```
 -tags=versionbump
 -tags=upgrademanual
 
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
- ${CMD}                   command to run
- ${VALUE}                 value to check on host
- ${INSTALLTYPE}           type of installation (version or commit) + desired value
- &{WORKLOADNAME}          name of the workload to deploy
- &{DESCRIPTION}           description of the test

Commands: 
$ make test-env-up                     # create the image from Dockerfile.build
$ make test-run                        # runs create and upgrade cluster by passing the argname and argvalue
$ make test-env-down                   # removes the image and container by prefix
$ make test-env-clean                  # removes instances and resources created by testcase
$ make test-logs                       # prints logs from container the testcase
$ make test-complete                   # clean resources + remove images + run testcase
$ make test-create                     # runs create cluster test locally
$ make test-upgrade                    # runs upgrade cluster test locally
$ make test-version-bump               # runs version bump test locally
$ make test-run                        # runs create and upgrade cluster by passing the argname and argvalue
$ make remove-tf-state                 # removes acceptance state dir and files
$ make test-suite                      # runs all testcase locally in sequence not using the same state
$ make vet-lint                        # runs go vet and go lint
```
### Examples with docker:
```
- Create an image tagged

$ make test-env-up TAGNAME=ubuntu


- Run upgrade cluster test with `${IMGNAME}` and  `${TAGNAME}`

$ make test-run IMGNAME=2 TAGNAME=ubuntu TESTDIR=upgradecluster INSTALLTYPE=INSTALL_K3S_VERSION=v1.26.2+k3s1


- Run create and upgrade cluster just adding `INSTALLTYPE` flag to upgrade

$ make test-run INSTALLTYPE=INSTALL_K3S_COMMIT=257fa2c54cda332e42b8aae248c152f4d1898218


- Run version bump test upgrading with commit id

$ make test-run IMGNAME=x \
TAGNAME=y \
TESTDIR=versionbump \
CMD="k3s --version, kubectl get image..." \
VALUE="v1.26.2+k3s1, v0.0.21" " \
INSTALLTYPE=INSTALL_K3S_COMMIT=257fa2c54cda332e42b8aae248c152f4d1898218 \ 
TESTCASE=TestLocalPathProvisionerStorage \
DEPLOYWORKLOAD=true \
WORKLOADNAME="someWorkload.yaml"


````
### Examples to run locally:
````
- Run create cluster test:

$ make test-create

- Run upgrade cluster test:

$ make test-upgrade-manual INSTALLTYPE=INSTALL_K3S_COMMIT=257fa2c54cda332e42b8aae248c152f4d1898218

- Run bump version with go test:

$go test -timeout=45m -v -tags=versionbump  ./entrypoint/versionbump/... \
-cmd "/var/lib/rancher/k3s/data/current/bin/cni, kubectl get pod test-pod -o yaml ; | grep -A2 annotations, k3s -v" \
-expectedValue "CNI plugins plugin v1.2.0-k3s1,1M, v1.26" \
-expectedValueUpgrade "CNI plugins plugin v1.2.0-k3s1,1M, v1.27" \
-installVersionOrCommit INSTALL_K3S_VERSION=v1.27.2+k3s1 \
-testCase "TestServiceClusterIp, TestLocalPathProvisionerStorage" \
-deployWorkload true \
-workloadName "bandwidth-annotations.yaml"


 - Logs from test

$ make tf-logs IMGNAME=1

- Run lint for a specific directory

$ make vet-lint TESTDIR=upgradecluster

````

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

### Debugging
````
To focus individual runs on specific test clauses, you can prefix with `F`. For example, in the [create cluster test](../tests/acceptance/entrypoint/createcluster_test.go), you can update the initial creation to be: `FIt("Starts up with no issues", func() {` in order to focus the run on only that clause.
Or use break points in your IDE.
````



### Custom Reporting: WIP

### Debugging:
````
The cluster and VMs can be retained after a test by passing `-destroy=false`. 
To focus individual runs on specific test clauses, you can prefix with `F`. For example, in the [create cluster test](../tests/terraform/cases/createcluster_test.go), you can update the initial creation to be: `FIt("Starts up with no issues", func() {` in order to focus the run on only that clause.