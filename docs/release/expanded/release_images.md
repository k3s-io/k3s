# Create Release Images

## Create System Agent Installer Images

The k3s-io/k3s Release CI should dispatch the rancher/system-agent-installer-k3s repo, generating a tag there and triggering the CI to build images.
The system-agent-installer-k3s repository is used with Rancher v2prov system.
This often fails! Check the CI and if it was not triggered do the following:

After RCs are cut you need to manually release the system agent installer k3s, this along with KDM PR allows QA to fully test RCs.
This should happen directly after the KDM PR is generated, within a few hours of the release candidate being cut.
These images depend on the release artifact and can not be generated until after the k3s-io/k3s release CI completes.

1. Create a release in the system-agent-installer-k3s repo
   1. it should exactly match the release title in the k3s repo
   1. the target is "main" for all releases (no branches)
   1. no description
   1. make sure to check the "pre-release" checkbox
1. Watch the Drone Publish CI, it should be very quick
1. Verify that the new images appear in Docker hub

## Create K3S Upgrade Images

The k3s-io/k3s Release CI should dispatch the k3s-io/k3s-upgrade repo, generating a tag there and triggering the CI to build images.
These images depend on the release artifact and can not be generated until after the k3s-io/k3s release CI completes.
This sometimes fails! Check the CI and if it was not triggered do the following:

1. Create a release in the system-agent-installer-k3s repo
   1. it should exactly match the release title in the k3s repo
   1. the target is "main" for all releases (no branches)
   1. no description
   1. make sure to check the "pre-release" checkbox
1. Watch the Drone Publish CI, it should be very quick
1. Verify that the new images appear in Docker hub

Make sure you are in constant communication with QA during this time so that you can cut more RCs if necessary,
 update KDM if necessary, radiate information to the rest of the team and help them in any way possible.
