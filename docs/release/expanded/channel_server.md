# Update Channel Server

Once the release is verified, the channel server config needs to be updated to reflect the new version for “stable”.  

1. Channel.yaml can be found at the [root of the K3s repo.](https://github.com/k3s-io/k3s/blob/master/channel.yaml)
   1. When updating the channel server a single-line change will need to be performed.  
   1. Release Captains responsible for this change will need to update the following stanza to reflect the new stable version of kubernetes relative to the release in progress.  
   1. Example:
      ```
      channels:
        name: stable
        latest: v1.22.12+k3s1
      ```
