# Set Up Environment Variables

The scripts and tools involved in release require specific environment variables,
 the value of these variables is not always obvious.
This guide helps you navigate the creation of those variables.


1. set GLOBAL_GIT_CONFIG_PATH environment variable (to the path of your git config, ex. '$HOME/.gitconfig'), this will be mounted into a docker container
1. set SSH_MOUNT_POINT environment variable (to the path of your SSH_AUTH_SOCK or your ssh key), this will be mounted into a docker container
1. set OLD_K8S to the previous k8s version
1. set NEW_K8S to the newly released k8s version
1. set OLD_K8S_CLIENT to the kubernetes/go-client version which corresponds with the previous k8s version
1. set NEW_K8S_CLIENT to the client version which corresponds with the newly released k8s version
1. set OLD_K3S_VER to the previous k3s version (the one which corresponds to the previous k8s version), replacing the plus symbol with a dash (eg. for "v1.25.0+k3s1" use "v1.25.0-k3s1")
1. set NEW_K3S_VER to the k3s version which corresponds to the newly released k8s version, replacing the plus symbol with a dash
1. set RELEASE_BRANCH to the k3s release branch which corresponds to the newly released k8s version
1. set GOPATH to the path to the "go" directory (usually $HOME/go)
1. set GOVERSION to the version of go which the newly released k8s version uses
   1. you can find this in the kubernetes/kubernetes repo
   1. go to the release tag in the proper release branch
   1. go to the build/dependencies.yaml
   1. search for the "golang: upstream version" stanza and the go version is the "version" in that stanza
   1. example: https://github.com/kubernetes/kubernetes/blob/v1.25.1/build/dependencies.yaml#L90-L91
1. set GOIMAGE to the go version followed by the alpine container version
   1. example: "golang:1.16.15-alpine"
   1. the first part correlates to the go version in this example the GOVERSION would be '1.16.15'
   1. the second part is usually the same "-alpine"
1. set BUILD_CONTAINER to the contents of a Dockerfile to build the "build container" for generating the tags
   1. the FROM line is the GOIMAGE
   1. the only other line is a RUN which adds a few utilities: "bash git make tar gzip curl git coreutils rsync alpine-sdk"
   1. example: BUILD_CONTAINER="FROM golang:1.16.15-alpine\n RUN apk add --no-cache bash git make tar gzip curl git coreutils rsync alpine-sdk"
1. I like to set this to a file and source it, it helps in case you need to set it again or to see what you did
   1. example:
      ```
      export SSH_MOUNT_PATH="/var/folders/m7/1d53xcj57d76n1qxv_ykgr040000gp/T//ssh-dmtrX2MOkrzO/agent.45422"
  
      export OLD_K8S="v1.22.13"
      export NEW_K8S="v1.22.14"
      export OLD_K8S_CLIENT="v0.22.13"
      export NEW_K8S_CLIENT="v0.22.14"
      export OLD_K3S_VER="v1.22.13-k3s1" 
      export NEW_K3S_VER="v1.22.14-k3s1"
      export RELEASE_BRANCH="release-1.22"
      export GOVERSION="1.16.15"
      export GOIMAGE="golang:1.16.15-alpine"
      # On Linux
      export GLOBAL_GIT_CONFIG_PATH="$HOME/.gitconfig"
      export GOPATH="$HOME/go"
      # On Mac
      export GLOBAL_GIT_CONFIG_PATH="/Users/mtrachier/.gitconfig"
      export GOPATH="/Users/mtrachier/go"
      
      export BUILD_CONTAINER="FROM golang:1.16.15-alpine\n RUN apk add --no-cache bash gnupg git make tar gzip curl git coreutils rsync alpine-sdk"
      ```