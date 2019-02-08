<!-- TOC -->

- [v1.13.0](#v1130)
  - [CRI CLI (crictl)](#cri-cli-crictl)
  - [CRI validation testing (critest)](#cri-validation-testing-critest)
  - [Downloads](#downloads)
- [v1.12.0](#v1120)
  - [CRI CLI (crictl)](#cri-cli-crictl-1)
  - [CRI validation testing (critest)](#cri-validation-testing-critest-1)
  - [Downloads](#downloads-1)
- [v1.11.1](#v1111)
  - [CRI CLI (crictl)](#cri-cli-crictl-2)
- [v1.11.0](#v1110)
  - [CRI validation testing (critest)](#cri-validation-testing-critest-2)
  - [CRI CLI (crictl)](#cri-cli-crictl-3)
- [v1.0.0-beta.1](#v100-beta1)
  - [CRI validation testing (critest)](#cri-validation-testing-critest-3)
  - [CRI CLI (crictl)](#cri-cli-crictl-4)
- [v1.0.0-beta.0](#v100-beta0)
  - [CRI validation testing (critest)](#cri-validation-testing-critest-4)
  - [CRI CLI (crictl)](#cri-cli-crictl-5)
- [v1.0.0-alpha.0](#v100-alpha0)
  - [CRI validation testing (critest)](#cri-validation-testing-critest-5)
  - [CRI CLI (crictl)](#cri-cli-crictl-6)
- [v0.2](#v02)
  - [CRI validation testing (critest)](#cri-validation-testing-critest-6)
  - [CRI CLI (crictl)](#cri-cli-crictl-7)
- [v0.1](#v01)
  - [Features](#features)
    - [CRI validation testing](#cri-validation-testing)
    - [crictl](#crictl)
    - [CRI performance benchmarking](#cri-performance-benchmarking)
  - [Documentation](#documentation)

<!-- /TOC -->

# v1.13.0

cri-tools v1.13.0 has upgraded to kubernetes v1.13. It mainly focus on bug fixes and stability improvements.

## CRI CLI (crictl)

- [#390](https://github.com/kubernetes-sigs/cri-tools/pull/390) Adds `--auth` options for pull command.
- [#392](https://github.com/kubernetes-sigs/cri-tools/pull/392) Fixes URL parsing for exec and attach.
- [#393](https://github.com/kubernetes-sigs/cri-tools/pull/393) Upgrades Go version to 1.11.1.
- [#394](https://github.com/kubernetes-sigs/cri-tools/pull/394) Enables Windows CI on travis.
- [#398](https://github.com/kubernetes-sigs/cri-tools/pull/398) Switches Windows default endpoints to npipe.
- [#402](https://github.com/kubernetes-sigs/cri-tools/pull/402) Updates version matrix information for the project.
- [#404](https://github.com/kubernetes-sigs/cri-tools/pull/404) Adds container name filter to ps command.
- [#406](https://github.com/kubernetes-sigs/cri-tools/pull/406) Adds metadata to filters.
- [#407](https://github.com/kubernetes-sigs/cri-tools/pull/407) Prints annotations and labels for inspect command.

## CRI validation testing (critest)

- [#391](https://github.com/kubernetes-sigs/cri-tools/pull/391) Add tests for multiple containers in a pod.
- [#405](https://github.com/kubernetes-sigs/cri-tools/pull/405) Adds runtime handler support for critest.
- [#410](https://github.com/kubernetes-sigs/cri-tools/pull/410) Adds exec sync timeout test cases.
- [#411](https://github.com/kubernetes-sigs/cri-tools/pull/411) Fixes readonly filesystem test cases.

## Downloads

| file                                | sha256                                                       |
| ----------------------------------- | ------------------------------------------------------------ |
| [crictl-v1.13.0-linux-386.tar.gz](https://github.com/kubernetes-sigs/cri-tools/releases/download/v1.13.0/crictl-v1.13.0-linux-386.tar.gz)     | 8a289d86b97f678fd5ddbd973503f772cfab9c29ef5e391930130c6214feecc9 |
| [crictl-v1.13.0-linux-amd64.tar.gz](https://github.com/kubernetes-sigs/cri-tools/releases/download/v1.13.0/crictl-v1.13.0-linux-amd64.tar.gz)   | 9bdbea7a2b382494aff2ff014da328a042c5aba9096a7772e57fdf487e5a1d51 |
| [crictl-v1.13.0-linux-arm64.tar.gz](https://github.com/kubernetes-sigs/cri-tools/releases/download/v1.13.0/crictl-v1.13.0-linux-arm64.tar.gz)   | 68949c0cb5a37e7604c145d189cf1e109c08c93d9c710ba663db026b9c6f2746 |
| [crictl-v1.13.0-linux-arm.tar.gz](https://github.com/kubernetes-sigs/cri-tools/releases/download/v1.13.0/crictl-v1.13.0-linux-arm.tar.gz)     | 2e478ebed85f9d70d49fd8f1d1089c8fba6e37d3461aeef91813f1ab0f0df586 |
| [crictl-v1.13.0-linux-ppc64le.tar.gz](https://github.com/kubernetes-sigs/cri-tools/releases/download/v1.13.0/crictl-v1.13.0-linux-ppc64le.tar.gz) | e85c3f95afd9752c65ec5d94a374a33e80576548ce95c2771a0973d7e3d9e6fa |
| [crictl-v1.13.0-linux-s390x.tar.gz](https://github.com/kubernetes-sigs/cri-tools/releases/download/v1.13.0/crictl-v1.13.0-linux-s390x.tar.gz)   | fe623c98ddff7e4b8679169bc9bb222d1c5dc81867234f95e9966dcd410e7b6b |
| [crictl-v1.13.0-windows-386.tar.gz](https://github.com/kubernetes-sigs/cri-tools/releases/download/v1.13.0/crictl-v1.13.0-windows-386.tar.gz)   | 641db1383708735d00a82fa947cc43850eb1a80de7129120967af59b24c2cf13 |
| [crictl-v1.13.0-windows-amd64.tar.gz](https://github.com/kubernetes-sigs/cri-tools/releases/download/v1.13.0/crictl-v1.13.0-windows-amd64.tar.gz) | 1a8468d4b67f8f73b05d38e7df146160033561b25fe7e2cee7d3aa374842e72c |
| [critest-v1.13.0-linux-386.tar.gz](https://github.com/kubernetes-sigs/cri-tools/releases/download/v1.13.0/critest-v1.13.0-linux-386.tar.gz)    | 020f3dea6a6360655b85c2180a8958aab9ae458d33cb50d12ac1faa329704aac |
| [critest-v1.13.0-linux-amd64.tar.gz](https://github.com/kubernetes-sigs/cri-tools/releases/download/v1.13.0/critest-v1.13.0-linux-amd64.tar.gz)  | 0161bbaf1a891fc87a852659da103165fa788aa773a32fa2a1ed584b5dd04d99 |
| [critest-v1.13.0-linux-arm64.tar.gz](https://github.com/kubernetes-sigs/cri-tools/releases/download/v1.13.0/critest-v1.13.0-linux-arm64.tar.gz)  | 76ad6796aa1bcff6412d18b45ee4015f32b9cd96589704af414930ddeb7dff91 |
| [critest-v1.13.0-linux-arm.tar.gz](https://github.com/kubernetes-sigs/cri-tools/releases/download/v1.13.0/critest-v1.13.0-linux-arm.tar.gz)    | fb8ff0a90cd59f18878cb81b40dd2b4223973d068d9a5c484de4f8f3224d249e |

# v1.12.0

cri-tools v1.12.0 has upgraded to kubernetes v1.12. It mainly focus on bug fixes and new features introduced in kubernetes v1.12. It has also moved to <https://github.com/kubernetes-sigs/cri-tools>.

## CRI CLI (crictl)

- [#345](https://github.com/kubernetes-sigs/cri-tools/pull/345) Fixes missing Windows library
- [#354](https://github.com/kubernetes-sigs/cri-tools/pull/354) Properly returns errors when the output format is not supported
- [#357](https://github.com/kubernetes-sigs/cri-tools/pull/357) Fixes version information and install guides
- [#361](https://github.com/kubernetes-sigs/cri-tools/pull/361) Show concise image info for crictl ps
- [#363](https://github.com/kubernetes-sigs/cri-tools/pull/363) Fixes crictl ps and crictl pods
- [#367](https://github.com/kubernetes-sigs/cri-tools/pull/367) Fixes version information for release scripts
- [#369](https://github.com/kubernetes-sigs/cri-tools/pull/369) Adds podID in output of `crictl ps`
- [#370](https://github.com/kubernetes-sigs/cri-tools/pull/370) Fixes non JSON keys support in info map
- [#374](https://github.com/kubernetes-sigs/cri-tools/pull/374) Adds support for Windows npipe `\.\pipe\dockershim`
- [#375](https://github.com/kubernetes-sigs/cri-tools/pull/375) Adds sandbox config to `image pull`
- [#378](https://github.com/kubernetes-sigs/cri-tools/pull/378) Fixes unmarshal issues in `crictl inspecti`
- [#383](https://github.com/kubernetes-sigs/cri-tools/pull/383) Adds support for runtime handler
- [#384](https://github.com/kubernetes-sigs/cri-tools/pull/384) Fixes timeout for grpc dialer

## CRI validation testing (critest)

- [#377](https://github.com/kubernetes-sigs/cri-tools/pull/377) Adds new test to critest for privileged container

## Downloads

| file                                | sha256                                                       |
| ----------------------------------- | ------------------------------------------------------------ |
| [crictl-v1.12.0-linux-386.tar.gz](https://github.com/kubernetes-sigs/cri-tools/releases/download/v1.12.0/crictl-v1.12.0-linux-386.tar.gz)     | 028ccea08422e011fcf11db4ebed772b1c434b44c4dd717cecd80bd0d1e57417 |
| [crictl-v1.12.0-linux-amd64.tar.gz](https://github.com/kubernetes-sigs/cri-tools/releases/download/v1.12.0/crictl-v1.12.0-linux-amd64.tar.gz)   | e7d913bcce40bf54e37ab1d4b75013c823d0551e6bc088b217bc1893207b4844 |
| [crictl-v1.12.0-linux-arm64.tar.gz](https://github.com/kubernetes-sigs/cri-tools/releases/download/v1.12.0/crictl-v1.12.0-linux-arm64.tar.gz)   | 8466f08b59bf36d2eebcb9428c3d4e6e224c3065d800ead09ad730ce374da6fe |
| [crictl-v1.12.0-linux-arm.tar.gz](https://github.com/kubernetes-sigs/cri-tools/releases/download/v1.12.0/crictl-v1.12.0-linux-arm.tar.gz)     | ca6b4ac80278d32d9cc8b8b19de140fd1cc35640f088969f7068fea2df625490 |
| [crictl-v1.12.0-linux-ppc64le.tar.gz](https://github.com/kubernetes-sigs/cri-tools/releases/download/v1.12.0/crictl-v1.12.0-linux-ppc64le.tar.gz) | ec6254f1f6ffa064ba41825aab5612b7b005c8171fbcdac2ca3927d4e393000f |
| [crictl-v1.12.0-linux-s390x.tar.gz](https://github.com/kubernetes-sigs/cri-tools/releases/download/v1.12.0/crictl-v1.12.0-linux-s390x.tar.gz)   | 814aa9cd496be416612c2653097a1c9eb5784e38aa4889034b44ebf888709057 |
| [crictl-v1.12.0-windows-386.tar.gz](https://github.com/kubernetes-sigs/cri-tools/releases/download/v1.12.0/crictl-v1.12.0-windows-386.tar.gz)   | 4520520b106b232a8a6e99ecece19a83bf58b94d48e28b4c0483a4a0f59fe161 |
| [crictl-v1.12.0-windows-amd64.tar.gz](https://github.com/kubernetes-sigs/cri-tools/releases/download/v1.12.0/crictl-v1.12.0-windows-amd64.tar.gz) | e401db715a9f843acaae40846a4c18f6938df95c34d06af08aac2fc3e591b2a7 |
| [critest-v1.12.0-linux-386.tar.gz](https://github.com/kubernetes-sigs/cri-tools/releases/download/v1.12.0/critest-v1.12.0-linux-386.tar.gz)    | ae9da4a95147e1486575d649b4384e91ba701a0aecadbc91c70ea3a963ba1b6b |
| [critest-v1.12.0-linux-amd64.tar.gz](https://github.com/kubernetes-sigs/cri-tools/releases/download/v1.12.0/critest-v1.12.0-linux-amd64.tar.gz)  | 681055657a19b8ce2ecb2571e71cc7b069f33847f2f5ae72e220f55292a5e976 |
| [critest-v1.12.0-linux-arm64.tar.gz](https://github.com/kubernetes-sigs/cri-tools/releases/download/v1.12.0/critest-v1.12.0-linux-arm64.tar.gz)  | b3eb282ab6d845e8c640c51aa266dc9d373d991a824cf550fbc12c36f98dcc5d |
| [critest-v1.12.0-linux-arm.tar.gz](https://github.com/kubernetes-sigs/cri-tools/releases/download/v1.12.0/critest-v1.12.0-linux-arm.tar.gz)    | 4593d86afffa373ab2ec5ae3b66fc0ca5413db3dd8268603e13a4820e0f8633d |

# v1.11.1

cri-tools v1.11.1 mainly focused on UX improvement and bug fix.

## CRI CLI (crictl)

- [#338](https://github.com/kubernetes-sigs/cri-tools/pull/338) Allow filtering the pods with prefix matching of name and namespace
- [#342](https://github.com/kubernetes-sigs/cri-tools/pull/342) Clarify flag description in `crictl ps` and `crictl pods`.
- [#343](https://github.com/kubernetes-sigs/cri-tools/pull/343) Better terminal support in `crictl exec` and `crictl attach`, which also fixes issue [#288](https://github.com/kubernetes-sigs/cri-tools/issues/288) and [#181](https://github.com/kubernetes-sigs/cri-tools/issues/181).

# v1.11.0

cri-tools v1.11.0 mainly focused on stability improvements and multi-arch support. Container runtime interface (CRI) has been updated to v1alpha2 in order to be compatible with kubernetes v1.11.

## CRI validation testing (critest)

- [#300](https://github.com/kubernetes-sigs/cri-tools/pull/300) Make image-user test images multi-arch.
- [#311](https://github.com/kubernetes-sigs/cri-tools/pull/311) Adds push-manifest into all target in the image-user Makefile.
- [#313](https://github.com/kubernetes-sigs/cri-tools/pull/313) Make hostnet-nginx test images multi-arch.
- [#315](https://github.com/kubernetes-sigs/cri-tools/pull/315) Makes image-test test images multi-arch.
- [#320](https://github.com/kubernetes-sigs/cri-tools/pull/320) Adds container host path validation tests.

## CRI CLI (crictl)

- [#306](https://github.com/kubernetes-sigs/cri-tools/pull/306) Fixes argument parsing for crictl exec.
- [#312](https://github.com/kubernetes-sigs/cri-tools/pull/312) Fixes a typo in inspecti usage.
- [#316](https://github.com/kubernetes-sigs/cri-tools/pull/316) Cleanups container and sandbox state.
- [#321](https://github.com/kubernetes-sigs/cri-tools/pull/321) Improves documentation and examples of crictl.
- [#325](https://github.com/kubernetes-sigs/cri-tools/pull/325) Upgrades kubernetes vendor to v1.11 branch.

# v1.0.0-beta.1

cri-tools v1.0.0-beta.1 mainly focused on critest coverage improvement, and bug fixes.

## CRI validation testing (critest)

- [#282](https://github.com/kubernetes-sigs/cri-tools/pull/282) Add RunAsGroup test. The test `runtime should return error if RunAsGroup is set without RunAsUser` only works with Kubernetes 1.11+.
- [#289](https://github.com/kubernetes-sigs/cri-tools/pull/289) Add host network pod portforward test.
- [#290](https://github.com/kubernetes-sigs/cri-tools/pull/290) Use busybox:1.28 instead of busybox:1.26 in the test to better support multi-arch.
- [#296](https://github.com/kubernetes-sigs/cri-tools/pull/296) Make `critest` binary statically linked.

## CRI CLI (crictl)

- [#278](https://github.com/kubernetes-sigs/cri-tools/pull/278) Remove "sandbox" from `crictl` command description.
- [#279](https://github.com/kubernetes-sigs/cri-tools/pull/279) Remove `oom-score-adj` flag from `crictl update` because it is not supported by `runc`.
- [#291](https://github.com/kubernetes-sigs/cri-tools/pull/291) Fix a bug that `crictl` generates a log file in `/tmp` directory each run. This can potentially fill `/tmp` directory.
- [#296](https://github.com/kubernetes-sigs/cri-tools/pull/296) Make `crictl` binary statically linked.

# v1.0.0-beta.0

cri-tools v1.0.0-beta.0 is mainly focus on UX improvements, including make crictl command more user friendly and add initial Windows support. Container runtime interface (CRI) has been updated to v1alpha2 in order to be compatible with kubernetes v1.10. Version matrix and branches for different kubernetes versions are also added.

## CRI validation testing (critest)

- [#227](https://github.com/kubernetes-sigs/cri-tools/pull/227) Set StdinOnce to true for attach test
- [#232](https://github.com/kubernetes-sigs/cri-tools/pull/232) Improves CRI log parser
- [#242](https://github.com/kubernetes-sigs/cri-tools/pull/242) Add validation of reopening container logs
- [#250](https://github.com/kubernetes-sigs/cri-tools/pull/250) Add validation of username not empty in ImageStatus
- [#252](https://github.com/kubernetes-sigs/cri-tools/pull/252) Improve image test and make test run in parallel
- [#257](https://github.com/kubernetes-sigs/cri-tools/pull/257) Add golang 1.10 and fix a race condition
- [#261](https://github.com/kubernetes-sigs/cri-tools/pull/261) [#273](https://github.com/kubernetes-sigs/cri-tools/pull/273) Remove dependency of source code
- [#267](https://github.com/kubernetes-sigs/cri-tools/pull/267) Add test for pid namespace
- [#269](https://github.com/kubernetes-sigs/cri-tools/pull/269) Add validation of tty settings for exec

## CRI CLI (crictl)

- [#222](https://github.com/kubernetes-sigs/cri-tools/pull/222) Rename `sandboxes` subcommand to `pods` and rename`sandbox` to `podsandbox` in all subcommands
- [#225](https://github.com/kubernetes-sigs/cri-tools/pull/225) Add support of windows
- [#238](https://github.com/kubernetes-sigs/cri-tools/pull/238) Update CRI to v1alpha2
- [#255](https://github.com/kubernetes-sigs/cri-tools/pull/255) Add support of multiple Ids to subcommands
- [#256](https://github.com/kubernetes-sigs/cri-tools/pull/256) Add `crictl ps -q`
- [#258](https://github.com/kubernetes-sigs/cri-tools/pull/258) Rename CRI endpoints environment variable to `CONTAINER_RUNTIME_ENDPOINT` and `IMAGE_SERVICE_ENDPOINT`
- [#268](https://github.com/kubernetes-sigs/cri-tools/pull/268) Avoid panic when runtimes are using truncated IDs
- [#274](https://github.com/kubernetes-sigs/cri-tools/pull/274) Add support of insecure TLS without auth

# v1.0.0-alpha.0

cri-tools v1.0.0-alpha.0 is mainly focus on UX improvements, including make crictl command more user friendly and add more subcommands. It also updates container runtime interface (CRI) to kubernetes v1.9 and fixes bugs in validation test suites.

## CRI validation testing (critest)

- [#164](https://github.com/kubernetes-sigs/cri-tools/pull/164) Fix security context test to not rely on `/etc/hosts`
- [#165](https://github.com/kubernetes-sigs/cri-tools/pull/165) Validate IPv4 only for port mapping tests
- [#196](https://github.com/kubernetes-sigs/cri-tools/pull/196) Fix privileged container validation by replacing `ip link` with `brctl addbr` command
-  [#197](https://github.com/kubernetes-sigs/cri-tools/pull/197) Fix hostIPC validation to support old ipcmk versions
- [#199](https://github.com/kubernetes-sigs/cri-tools/pull/199) [#201](https://github.com/kubernetes-sigs/cri-tools/pull/201) Fix container logs validation
- [#200](https://github.com/kubernetes-sigs/cri-tools/pull/200) Add SELinux validation tests

## CRI CLI (crictl)

- [#156](https://github.com/kubernetes-sigs/cri-tools/pull/156) Fix empty RepoTags handling for `images` command
- [#163](https://github.com/kubernetes-sigs/cri-tools/pull/163) Add `--digest` option to `images` command
- [#167](https://github.com/kubernetes-sigs/cri-tools/pull/167) Add verbose for `status` command
- [#171](https://github.com/kubernetes-sigs/cri-tools/pull/171) Sort results by creation time for `ps`, `sandboxes` and `images` commands
- [#174](https://github.com/kubernetes-sigs/cri-tools/pull/174) Support select sandboxes by name for `sandboxes` and other commands
- [#178](https://github.com/kubernetes-sigs/cri-tools/pull/178) [#190](https://github.com/kubernetes-sigs/cri-tools/pull/190) Replace golang json with `protobuf/jsonpb` library
- [#182](https://github.com/kubernetes-sigs/cri-tools/pull/182) Fix stdout and stderr for `attach` and `exec` command
- [#183](https://github.com/kubernetes-sigs/cri-tools/pull/183) Add created time to `sandboxes` command
- [#186](https://github.com/kubernetes-sigs/cri-tools/pull/186) Use kubelet's log library instead of a copied one
- [#187](https://github.com/kubernetes-sigs/cri-tools/pull/187) Add image tag and attempt to `ps` command
- [#194](https://github.com/kubernetes-sigs/cri-tools/pull/194) Add `config` command
- [#217](https://github.com/kubernetes-sigs/cri-tools/pull/217) Add `--latest` and `--last` options to `ps` and `sandboxes` commands
- [#202](https://github.com/kubernetes-sigs/cri-tools/pull/202) [#203](https://github.com/kubernetes-sigs/cri-tools/pull/203) Add `--all`, `--latest`, `--last` and `--no-trunc` options to `ps` command
- [#205](https://github.com/kubernetes-sigs/cri-tools/pull/205) Improve logs command and add `--timestamps` and `--since` options
- [#206](https://github.com/kubernetes-sigs/cri-tools/pull/206) Add verbose debut output to `inspect` and `inspects` commands
- [#207](https://github.com/kubernetes-sigs/cri-tools/pull/207) Sort flags for all commands
- [#209](https://github.com/kubernetes-sigs/cri-tools/pull/209) Add `stats` command
- [#211](https://github.com/kubernetes-sigs/cri-tools/pull/211) Rewrite timestamps in container status and sandbox status to make them more user friendly
- [#213](https://github.com/kubernetes-sigs/cri-tools/pull/213) Add completion command
- [#216](https://github.com/kubernetes-sigs/cri-tools/pull/216) Add `--no-trunc` to `images` and `sandboxes` commands

# v0.2

cri-tools v0.2 enhances validation testings, improves crictl UX and also fixes several bugs.  It has also updates container runtime interface (CRI) to kubernetes v1.8.

## CRI validation testing (critest)

- [#127](https://github.com/kubernetes-sigs/cri-tools/pull/127) Adds validation tests for supplemental groups
- [#135](https://github.com/kubernetes-sigs/cri-tools/pull/135) [#137](https://github.com/kubernetes-sigs/cri-tools/pull/137) and [#144](https://github.com/kubernetes-sigs/cri-tools/pull/144) Adds validation tests for seccomp
- [#139](https://github.com/kubernetes-sigs/cri-tools/pull/139) Adds validation tests for sysctls
- [#140](https://github.com/kubernetes-sigs/cri-tools/pull/140) Adds validation tests for AppArmor
- [#141](https://github.com/kubernetes-sigs/cri-tools/pull/141) Adds validation tests for NoNewPrivs
- [#142](https://github.com/kubernetes-sigs/cri-tools/pull/142) Adds validation tests for mount propagation
- [#115](https://github.com/kubernetes-sigs/cri-tools/pull/115) Fixes image validation tests
- [#116](https://github.com/kubernetes-sigs/cri-tools/pull/116) Fixes validation message
- [#126](https://github.com/kubernetes-sigs/cri-tools/pull/126) Fixes sandbox leak in port forward validation tests

## CRI CLI (crictl)

- [#122](https://github.com/kubernetes-sigs/cri-tools/pull/122) Adds support for authenticated image pull
- [#123](https://github.com/kubernetes-sigs/cri-tools/pull/123) Improves crictl UX
- [#124](https://github.com/kubernetes-sigs/cri-tools/pull/124) Adds support for creating sandboxes and containers from yaml
- [#133](https://github.com/kubernetes-sigs/cri-tools/pull/133) Adds timeout support for container stop

# v0.1

cri-tools provides a set of tools for Kubelet Container Runtime Interface (CRI):

- **CRI validation testing**
  - provides a test framework and a suite of tests to validate that the Container Runtime Interface (CRI) server implementation meets all the requirements.
  - allows the CRI runtime developers to verify that their runtime conforms to CRI, without needing to set up Kubernetes components or run Kubernetes end-to-end tests.
- **crictl**
  - provides a CLI for CRI-compatible container runtimes.
  - allows the CRI runtime developers to debug of their runtime without needing to set up Kubernetes components.
- **CRI performance benchmarking**
  - provides a benchmarking framework for CRI-compatible container runtimes.
  - allows the CRI runtime developers to benchmark the performance of their runtime without needing to set up Kubernetes components or run Kubernetes benchmark tests.

## Features

### CRI validation testing

  - basic sandbox and container operations
  - basic image operations
  - networking, e.g. DNS config, port mapping
  - streaming, e.g. exec, attach, portforward
  - security context, e.g.
    - hostPID, hostIPC, hostNetwork
    - runAsUser, readOnlyRootfs, privileged
  - execSync,version,status

### crictl

  - get version and status
  - sandbox run, stop, status, list, and remove
  - container create, start, stop, status, list and remove
  - image pull, list, status and remove
  - streaming attach, exec and portforward

### CRI performance benchmarking

  - parallel sandbox run, stop, status, list and remove
  - parallel container create, start, stop, status, list and remove
  - parallel image pull, list and remove

## Documentation

See [cri-tools](https://github.com/kubernetes-sigs/cri-tools/#documentation).
