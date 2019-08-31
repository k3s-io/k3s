# How to Create a New Release

## Prerequisites

Install [releasetool](https://github.com/googleapis/releasetool).

## Create a release

1. `cd` into the root directory, e.g., `~/go/src/cloud.google.com/go`
1. Checkout the master branch and ensure a clean and up-to-date state.
    ```
    git checkout master
    git pull --tags origin master
    ```
1. Run releasetool to generate a changelog from the last version. Note,
   releasetool will prompt if the new version is a major, minor, or patch
   version.
    ```
    releasetool start --language go
    ```
1. Format the output to match CHANGES.md.
1. Submit a CL with the changes in CHANGES.md. The commit message should look
   like this (where `v0.31.0` is instead the correct version number):
    ```
    all: Release v0.31.0
    ```
1. Wait for approval from all reviewers and then submit the CL.
1. Return to the master branch and pull the release commit.
    ```
    git checkout master
    git pull origin master
    ```
1. Tag the current commit with the new version (e.g., `v0.31.0`)
    ```
    releasetool tag --language go
    ```
1. Publish the tag to GoogleSource (i.e., origin):
    ```
    git push origin $NEW_VERSION
    ```
1. Visit the [releases page][releases] on GitHub and click the "Draft a new
   release" button. For tag version, enter the tag published in the previous
   step. For the release title, use the version (e.g., `v0.31.0`). For the
   description, copy the changes added to CHANGES.md.


[releases]: https://github.com/GoogleCloudPlatform/google-cloud-go/releases
