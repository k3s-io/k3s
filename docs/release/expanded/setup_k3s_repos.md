# Set Up K3S Repos

1. make sure the $HOME/go/src/github.com/k3s-io directory exists
1. clear out (remove) k3s repo if is already there (just makes things smoother with a new clone)
1. clone k3s-io/k3s repo into that directory as "upstream"
1. fork that repo so that you have a private fork of it
   1. if you already have a fork, sync it
1. add your fork repo as "origin"
1. fetch all objects from both repos into your local copy
   1. it is important to follow these steps because Go is very particular about the file structure (it uses the file structure to infer the urls it will pull dependencies from)
   1. this is why it is important that the repo is in the github.com/k3s-io directory, and that the repo's directory is "k3s" matching the upstream copy's name
`$HOME/go/src/github.com/k3s-io/k3s`