# Rebase

1. clear out any cached or old files: `git add -A; git reset --hard HEAD`
1. clear out any cached or older outputs: `rm -rf _output`
1. rebase your local copy to move the old k3s tag from the old k8s tag to the new k8s tag
   1. so there are three copies of the code involved in this process:
      1. the upstream kubernetes/kubernets copy on GitHub
      1. the k3s-io/kubernetes copy on GitHub
      1. and the local copy on your laptop which is a merge of those
   1. the local copy has every branch and every tag from the remotes you have added
   1. there are custom/proprietary commits in the k3s-io copy that are not in the kubernetes copy
   1. there are commits in the kubernetes copy do not exist in the k3s-io copy
   1. we want the new commits added to the kubernetes copy to be in the k3s-io copy
   1. we want the custom/proprietary commits from the k3s-io copy on top of the new kubernetes commits
   1. before rebase our local copy has all of the commits, but the custom/proprietary k3s-io commits are between the old kubernetes version and the new kubernetes version
   1. after the rebase our local copy will have the k3s-io custom/proprietary commits after the latest kubernetes commits
   1. `git rebase --onto $NEW_K8S $OLD_K8S $OLD_K3S_VER~1`
   1. After rebase you will be in a detached head state, this is normal