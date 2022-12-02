# Setup Go Environment

These steps are expected for using the scripts and ecm_distro tools for release.  
Some of these steps are for properly setting up Go on your machine, some for Docker, and Git.

## Git

1. install Git (using any method that makes sense
1. Configure Git for working with GitHub (add your ssh key, etc)

## Go

1. install Go from binary
1. set up default Go file structure
   1. create $HOME/go/src/github.com/<your user>
   1. create $HOME/go/src/github.com/k3s-io
   1. create $HOME/go/src/github.com/rancher
   1. create $HOME/go/src/github.com/rancherlabs
   1. create $HOME/go/src/github.com/kubernetes
1. set GOPATH=$HOME/go

## Docker

1. install Docker (or Docker desktop) using whatever method makes sense
