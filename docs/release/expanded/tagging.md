# Generate Kubernetes Tags

1. run the tag.sh script
   1. the tag.sh script is in the commits that exist in the k3s-io/kubernetes copy but not the kubernetes/kubernetes copy
   2. when we fetched all from both copies to our local copy we got the tag.sh
   3. when we rebased our local copy the tag.sh appears in HEAD
   4. the tag.sh requires a strict env to run in, which is why we generated the build container
   5. we can now run the tag.sh script in the docker container:
      ```
      docker run --rm -u $(id -u) --mount type=tmpfs,destination=${GOPATH}/pkg -v ${GOPATH}/src:/go/src -v ${GOPATH}/.cache:/go/.cache -v ${GLOBAL_GIT_CONFIG_PATH}:/go/.gitconfig -v ${HOME}/.gnupg:/go/.gnupg -e HOME=/go -e GOCACHE=/go/.cache -w /go/src/github.com/kubernetes/kubernetes ${GOIMAGE}-dev ./tag.sh ${NEW_K3S_VER} 2>&1 | tee tags-${NEW_K3S_VER}.log
      ```
2. the tag.sh script builds a lot of binaries and creates a commit in your name
   - this can take a while, like 45min in my case
3. the tag.sh script creates a lot of tags in the local copy
4. the "push" output from the tag.sh is a list of commands to be run. Always review the commits and tags that the tag.sh creates before pushing
5. build and run the push script
   1. there is a lot of output, but only about half of it are git push commands, only copy the "git push" commands
   2. after pasting the push commands to a file, make the file executable
   3. make sure you are able to push to the k3s-io/kubernetes repo, this is where you will be pushing the tags and commits
   4. Set the REMOTE env to k3s-io before running the script:
      ```
      export REMOTE=k3s-io
      ```
   5. Run the push script to add the tags from your local copy to the k3s-io/kubernetes copy