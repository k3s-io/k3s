# K3S Release Process

## Setup

Set up you environment per [setup](expanded/setup_env.md).

## Generate New Tags for K3S-IO/Kubernetes Fork

1. Generate specific environment variables per [setup rc](expanded/setup_rc.md).
1. Set up Kubernetes repos per [setup k8s repos](expanded/setup_k8s_repos.md).
1. Rebase your local copy to move the old k3s tag from the old k8s tag to the new k8s tag, per [rebase](expanded/rebase.md).
1. Build a custom container for generating tags, per [build container](expanded/build_container.md).
1. Run the tag script to generate tags in the build container, per [tagging](expanded/tagging.md).

## Update K3S

We made some new tags on the k3s-io/kubernetes repo, now we need to tell k3s to use them.

1. If no milestones exist in the k3s repo for the releases, generate them, per [milestones](expanded/milestones.md).
1. Set up k3s repos per [setup k3s repos](expanded/setup_k3s_repos.md).
1. Generate a pull request to update k3s, per [generate pull request](expanded/pr.md).

## Cut Release Candidate

1. The first part of cutting a release (either an RC or a GA) is to create the release itself, per [cut release](expanded/cut_release.md).
1. Then we need to update KDM, per [update kdm](expanded/update_kdm.md).
1. We check the release images, per [release images](expanded/release_images.md).
1. Then we need to update or generate the release notes, per [release notes](expanded/release_notes.md).

## Create GA Release

After QA approves the release candidates you need to cut the "GA" release.  
This will be tested one more time before the release is considered ready for finalization.  

Follow the processes for an RC release:
1. [Cut Release](expanded/cut_release.md)
1. [Update KDM](expanded/update_kdm.md)
1. [Check Release Images](expanded/release_images.md)
1. [Update Release Notes](expanded/release_notes.md)

Make sure you are in constant communication with QA during this time so that you can cut more RCs if necessary, 
 update KDM if necessary, radiate information to the rest of the team and help them in any way possible.  
When QA approves the GA release you can move into the finalization phase.

## Finalization

1. Update the channel server, per [channel server](expanded/channel_server.md)
1. Copy the release notes into the release, make sure Release Notes already merged, per [release notes](expanded/release_notes.md)
1. CI has completed, and artifacts have been created. Announce the GA and inform that k3s is thawed in the Slack release thread.
1. Wait 24 hours, then uncheck the pre-release checkbox on the release.
1. Edit the release, and check the "set as latest release" checkbox on the "latest" release.
   - only one release can be latest
   - this will most likely be the patch for the highest/newest minor version
   - check with QA for which release this should be
