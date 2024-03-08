# Set Up Kubernetes Repos

1. make sure the $HOME/go/src/github.com/kubernetes directory exists
1. clear out (remove) kubernetes repo if is already there (just makes things smoother with a new clone)
1. clone kubernetes/kubernetes repo into that directory as "upstream"
1. add k3s-io/kubernetes repo as "k3s-io"
1. fetch all objects from both repos into your local copy
   1. it is important to follow these steps because Go is very particular about the file structure (it uses the file structure to infer the urls it will pull dependencies from)
   1. this is why it is important that the repo is in the github.com/kubernetes directory, and that the repo's directory is "kubernetes" matching the upstream copy's name `$HOME/go/src/github.com/kubernetes/kubernetes`
