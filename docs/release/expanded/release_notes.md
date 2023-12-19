# Create Release Notes PR

1. Use the release notes tool to generate the release notes
   1. the release notes tool is located in [ecm distro tools](https://github.com/rancher/ecm-distro-tools)
   1. you will need a valid GitHub token to use the tool
   1. call the tools as follows (example using v1.23.13-rc1+k3s1):
      ```
      # this outputs to stdout
      export GHT=$GITHUB_TOKEN
      export PREVIOUS_RELEASE='v1.23.12+k3s1'
      export LAST_RELEASE='v1.23.13-rc2+k3s1'
      docker run --rm -e GITHUB_TOKEN=$GHT rancher/ecm-distro-tools:latest gen_release_notes -r k3s -m $LAST_RELEASE -p $PREVIOUS_RELEASE
      ```
1. Update the first line to include the semver of the released version
   - example: `<!-- v1.25.3+k3s1 -->`
1. Make sure the title has the new k8s version, and the "changes since" line has the old version number
   - example title: `This release updates Kubernetes to v1.25.3, and fixes a number of issues.`
   - example "changes since": `## Changes since v1.25.2+k3s1`
1. Verify changes
   1. go to releases
   1. find the previous release
   1. calculate the actual date from "XX days ago"
   1. search for pull requests which merged after that date
   1. go to the GitHub issue search UI
   1. search for PRs for the proper branch, merged after the last release
      - release branch is release-1.23
      - previous release was v1.23.12
      - date of the release was "Sept 28th 2022"
      - example search `is:pr base:release-1.23 merged:>2022-09-28 sort:created-asc`
   1. for each PR, validate the title of the pr and the commit message comments
      - each PR title (or 'release note' section of the first comment) should get an entry
      - the entry with this item should have a link at the end to the PR
      - the commit messages for the PR should follow, until the next PR
   1. if you suspect there is a missing/extra commit, compare the tags
      - use the github compare tool to compare the older tag to the newer one
      - example: `https://github.com/k3s-io/k3s/compare/v1.25.2+k3s1...v1.25.3-rc2+k3s1`
        - this will show all of the commit differences between the two, exposing all of the new commits
      - on the commit page you should see the merge issue associated with it
      - validate that the merge issue is listed in the release notes
      - if the commit is not in the comparison, try comparing the previous release tags
        - example: `https://github.com/k3s-io/k3s/compare/v1.25.0+k3s1...v1.25.2+k3s1`
        - the commit's merge issue should be listed in the release notes
   1. if you are adding backports, make sure you are using the backport issues, not the one for master
1. Verify component release versions
   - the list of components is completely static, someone should say something in the PR if we need to add to the list
     - Kubernetes, Kine, SQLite, Etcd, Containerd, Runc, Flannel, Metrics-server, Traefik, CoreDNS, Helm-controller, Local-path-provisioner
   - the version.sh script found in the k3s repo at scripts/version.sh is the source of truth for version information
   1. go to [the k3s repo](https://github.com/k3s-io/k3s) and browse the release tag for the notes you are verifying, [example](https://github.com/k3s-io/k3s/blob/v1.23.13-rc2+k3s1)
   1. start by searching the version.sh file for the component
   1. if you do not find anything, search the build script found in ./scripts/build [example](https://github.com/k3s-io/k3s/blob/v1.23.13-rc2+k3s1/scripts/build)
   1. if you still do not find anything, search the go.mod found in the root of the k3s repo [example](https://github.com/k3s-io/k3s/blob/v1.23.13-rc2+k3s1/go.mod#L93)
   1. some things are in the k3s repo's manifests directory, see ./manifests [example](https://github.com/k3s-io/k3s/blob/v1.23.13-rc2%2Bk3s1/manifests/local-storage.yaml#L66)
   - example info for v1.23.13-rc2+k3s1
     ```
     kubernetes: version.sh pulls from k3s repo go.mod see https://github.com/k3s-io/k3s/blob/v1.23.13-rc2+k3s1/scripts/version.sh#L35
     kine: go.mod, see https://github.com/k3s-io/k3s/blob/v1.23.13-rc2+k3s1/go.mod#L93
     sqlite: go.mod, see https://github.com/k3s-io/k3s/blob/v1.23.13-rc2+k3s1/go.mod#L97
     etcd: go.mod, use the /api/v3 mod, see https://github.com/k3s-io/k3s/blob/v1.23.13-rc2+k3s1/go.mod#L25
     containerd: version.sh sets an env variable based on go.mod, then the build script builds it
       see https://github.com/k3s-io/k3s/blob/v1.23.13-rc2%2Bk3s1/scripts/version.sh#L25
       and https://github.com/k3s-io/k3s/blob/v1.23.13-rc2%2Bk3s1/scripts/build#L36
     runc: set in the version.sh
       this one is weird, it ignores the go.mod, preferring the version.sh instead
       the version.sh sets an env variable which is picked up by the download script
       the build script runs 'make' on whatever was downloaded
       see https://github.com/k3s-io/k3s/blob/v1.23.13-rc2%2Bk3s1/scripts/version.sh#L40
       and https://github.com/k3s-io/k3s/blob/v1.23.13-rc2+k3s1/scripts/download#L29
       and https://github.com/k3s-io/k3s/blob/master/scripts/build#L138
     flannel: version.sh sets an env variable based on go.mod, then the build script builds it
       see https://github.com/k3s-io/k3s/blob/v1.23.13-rc2+k3s1/go.mod#L83
     metrics-server: version is set in the manifest at manifests/metric-server
       see https://github.com/k3s-io/k3s/blob/v1.23.13-rc2+k3s1/manifests/metrics-server/metrics-server-deployment.yaml#L42
     traefik: version is set in the manifest at manifests/traefik.yaml
       see https://github.com/k3s-io/k3s/blob/v1.23.13-rc2%2Bk3s1/manifests/traefik.yaml#L36
     coredns: version is set in the manifest ar manifests/coredns.yaml
       see https://github.com/k3s-io/k3s/blob/v1.23.13-rc2%2Bk3s1/manifests/coredns.yaml#L122
     helm-controller: go.mod, see https://github.com/k3s-io/k3s/blob/v1.23.13-rc2%2Bk3s1/go.mod#L92
     local-path-provisioner: version is set in the manifest at manifests/local-storage.yaml
       see https://github.com/k3s-io/k3s/blob/v1.23.13-rc2%2Bk3s1/manifests/local-storage.yaml#L66
     ```

## Understanding Release Notes

Here are the major sections in the release notes:
- changes since
  - this relates all changes since the previous release
  - more specifically every merge issue (PR) generated should have an entry
  - developers may add a special "User-Facing Change" section to their PR to give custom notes
    - these notes will appear as sub entries on the issue title
- released components
  - this relates all kubernetes 'components' in the release
  - components are generally non-core kubernetes options that we install using Helm charts
