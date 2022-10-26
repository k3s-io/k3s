# Generate Pull Request

We update the go.mod in k3s to point to the new modules, and submit the change for review.

1. make sure git is clean before making changes
1. make sure your origin is up to date before making changes
1. checkout a new branch for the new k3s version in the local copy using the formal semantic name eg. "v1.25.1-k3s1"
1. replace any instances of the old k3s version eg. "v1.25.0-k3s1" with the new k3s version eg. "v1.25.1-k3s1" in k3,s-io module links
1. replace any instances of the old Kubernetes version eg. "v1.25.0" with the new Kubernetes version eg. "v1.25.1"
1. replace any instances of the old Kubernetes client-go version eg. "v0.25.0" with the new version eg. "v0.25.1"
1. sed commands make this process easier (this is not a script):
   1. Linux example:
      ```
      sed -Ei "\|github.com/k3s-io/kubernetes| s|${OLD_K3S_VER}|${NEW_K3S_VER}|" go.mod
      sed -Ei "s/k8s.io\/kubernetes v\S+/k8s.io\/kubernetes ${NEW_K8S}/" go.mod
      sed -Ei "s/$OLD_K8S_CLIENT/$NEW_K8S_CLIENT/g" go.mod
      ```
    1. Mac example:
       ```
       # note that sed has different parameters on MacOS than Linux
       # also note that zsh is the default MacOS shell and is not bash/dash (the default Linux shells)
       sed -Ei '' "\|github.com/k3s-io/kubernetes| s|${OLD_K3S_VER}|${NEW_K3S_VER}|" go.mod
       git diff

       sed -Ei '' "s/k8s.io\/kubernetes v.*$/k8s.io\/kubernetes ${NEW_K8S}/" go.mod
       git diff

       sed -Ei '' "s/${OLD_K8S_CLIENT}/${NEW_K8S_CLIENT}/g" go.mod
       git diff

       go mod tidy
       git diff
       ```
1. update extra places to make sure the go version is correct
   1. `.github/workflows/integration.yaml`
   1. `.github/workflows/unitcoverage.yaml`
   1. `Dockerfile.dapper`
   1. `Dockerfile.manifest`
   1. `Dockerfile.test`
1. commit the changes and push to your origin
   1. make sure to sign your commits
   1. make sure to push to "origin" not "upstream", be explicit in your push commands
   1. example: 'git push -u origin v1.25.1-k3s1'
1. the git output will include a link to generate a pull request, use it
   1. make sure the PR is against the proper release branch
1. generating the PR starts several CI processes, most are in GitHub actions, but some one is in Drone, post the link to the drone CI run in the PR
   1. this keeps everyone on the same page
   1. if there is an error in the CI, make sure to note that and what the errors are for reviewers
   1. finding error messages:
      1. example: https://drone-pr.k3s.io/k3s-io/k3s/4744
      1. click the "show all logs" to see all of the logs
      1. search for " failed." this will find a line like "Test bEaiAq failed."
      1. search for "err=" and look for a log with the id "bEaiAq" in it
      1. example error:
         ```
         #- Tail: /tmp/bEaiAq/agents/1/logs/system.log
         [LATEST-SERVER] E0921 19:16:55.430977      57 cri_stats_provider.go:455] "Failed to get the info of the filesystem with mountpoint" err="unable to find data in memory cache" mountpoint="/var/lib/rancher/k3s/agent/containerd/io.containerd.snapshotter.v1.overlayfs
         [LATEST-SERVER] I0921 19:16:55.431186      57 proxier.go:667] "Failed to load kernel module with modprobe, you can ignore this message when kube-proxy is running inside container without mounting /lib/modules" moduleName="ip_vs_rr"
         ```
      1. the first part of the log gives a hint to the log level: "E0921" is an error log "I0921" is an info log
      1. you can also look for "Summarizing \d Failure" (I installed a plugin on my browser to get regex search: "Chrome Regex Search")
      1. example error: 
         ```
         [Fail] [sig-network] DNS [It] should support configurable pod DNS nameservers [Conformance]
         ```
    1. example PR: https://github.com/k3s-io/k3s/pull/6164
    1. many errors are flakey/transitive, it is usually a good idea to simply retry the CI on the first failure
    1. if the same error occurs multiple times then it is a good idea to escalate to the team
1. After the CI passes (or the team dismisses the CI as "flakey"), and you have at least 2 approvals you can merge it
   1. make sure you have 2 approvals on the latest changes
   1. make sure the CI passes or the team approves merging without it passing
   1. make sure the use the "squash and merge" option in GutHub
   1. make sure to update the SLACK channel with the new Publish/Merge CI

- Help! My memory usage is off the charts and everything has slowed to a crawl!
  - I found rebooting after running tag.sh was the only way to solve this problem, seems like a memory leak in VSCode on Mac or maybe some weird behavior between all of the added/removed files along with VSCode's file parser, the Crowdstrike virus scanner, and Docker (my top memory users)
