# Generate Kubernetes Tags

1. run the tag.sh script
   1. the tag.sh script is in the commits that exist in the k3s-io/kubernetes copy but not the kubernetes/kubernetes copy
   1. when we fetched all from both copies to our local copy we got the tag.sh
   1. when we rebased our local copy the tag.sh appears in HEAD
   1. the tag.sh requires a strict env to run in, which is why we generated the build container
   1. we can now run the tag.sh script in the docker container
   1. `docker run --rm -u $(id -u) --mount type=tmpfs,destination=${GOPATH}/pkg -v ${GOPATH}/src:/go/src -v ${GOPATH}/.cache:/go/.cache -v ${GLOBAL_GIT_CONFIG_PATH}:/go/.gitconfig -e HOME=/go -e GOCACHE=/go/.cache -w /go/src/github.com/kubernetes/kubernetes ${GOIMAGE}-dev ./tag.sh ${NEW_K3S_VER} 2>&1 | tee tags-${NEW_K3S_VER}.log`
1. the tag.sh script builds a lot of binaries and creates a commit in your name
   1. this can take a while, like 45min in my case
1. the tag.sh script creates a lot of tags in the local copy
1. the "push" output from the tag.sh is a list of commands to be run
   1. you should review the commits and tags that the tag.sh creates
   1. always review automated commits before pushing
1. build and run the push script
   1. there is a lot of output, but only about half of it are git push commands, only copy the commands to build a "push" script
   1. after pasting the push commands to a file, make the file executable
   1. make sure you are able to push to the k3s-io/kubernetes repo, this is where you will be pushing the tags and commits
   1. make sure to set the REMOTE env variable to "k3s-io" before running the script
   1. the push script pushes up the tags and commits from your local copy to the k3s-io/kubernetes copy