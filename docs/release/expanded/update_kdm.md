# Update KDM

After the RCs are cut you need to generate the KDM PR within a few hours

## Set up Repo

1. make sure the $HOME/go/src/github.com/rancher directory exists
1. clear out (remove) kontainer-driver-metadata repo if is already there (just makes things smoother with a new clone)
1. fork kdm repo
1. clone your fork into that directory as "origin" (you won't need a local copy of upstream)
   1. it is important to follow these steps because Go is very particular about the file structure (it uses the file structure to infer the urls it will pull dependencies from)
   1. go generate needs to be able to fully use Go as expected, so it is important to get the file structure correct
   1. this is why it is important that the repo is in the github.com/rancher directory, and that the repo's directory is "kontainer-driver-metadata" matching the upstream copy's name
      1. $HOME/go/src/github.com/rancher/kontainer-driver-metadata
1. checkout a new branch (something like "k3s-release-september")

## Update The Channels

1. Edit the "channels.yaml" file in the root of the repo
   1. copy and paste the previous version's info directly below it
   1. if a version was skipped, there should be a comment stating that
   1. ask QA captain what the min and max channel server versions should be
   1. generate the change to the channel.yaml and commit it

## Go Generate

1. Generate json data changes
   1. as a separate commit, run the command `go generate`
   1. this will alter the data/data.json file
   1. commit this change by itself with the commit message "go generate" (exactly that message)
   1. push the changes to your fork

## Squashing Your Changes

ok, so you have all the commits and you are ready to go, suddenly someone asks you to squash all the changes to the channels.yaml and the data/data.json together.
The goal is to have 2 commits, one with all the changes to channels.yaml, and one with the changes to data/data.json.
They might also ask you to rebase from the upstream branch...

1. Rebasing from upstream: `git pull --rebase upstream <branch to rebase from>` for example: `git pull --rebase upstream dev-v2.7`
   1. this will pull in all of the commits from upstream's 'dev-v2.7' branch into your local copy
   1. this will rebase your local copy's history on top of that pull
   1. you will need to verify your files and force push your local copy to your origin copy `git push -f origin <branch name>`, for example: `git push -f origin k3s-release-september`
   1. you will see all of the commits for the PR re-added as part of this process, take a note of how many commits are in the PR (needed for next step)
   1. force push the rebase to your origin before moving to the next step, this will prevent a diverged head state.
1. Reset local copy: `git reset --hard HEAD~<commit number>`, for example if you had 20 commits: `git reset --hard HEAD~20`
   1. this resets your local copy to the point in git history just before your first commit
   1. before you reset make sure you are at the tip of HEAD (important for next step)
   1. look in the history in GitHub and verify that you are at the proper commit so that you don't squash anyone else's commits into your own
1. Pull in the commits after reset and squash them in your local copy: `git merge --squash HEAD@{1}`
   1. the `HEAD@{1}` is returning to where HEAD was before reset
   1. this does not actually make a commit for you, it only merges the commits into a single staged but uncommitted state
1. remove the data/data.json from the staged for commit files: `git restore --staged data/data.json`
   1. this does not actually restore anything, it simply moves the file from staged for commit to unstaged
   1. you want to commit the channels.yaml in a separate commit from the data.json
1. commit the channels.yaml changes
   1. this single commit will replace any/all of the previous commits
   1. I put a message like "updating channels"
1. stage the data/data.json: `git add data/data.json`
   1. this adds a new commit with just the changes to the data.json, replacing the previous commits
   1. make sure the commit message is `go generate`
1. force push the changes to your origin
   1. don't force push to upstream!
   1. `git push -f origin <branch>` for example: `git push -f origin k3s-release-september`

## Create Pull Request

1. generate a PR against the default branch of the KDM repo
1. Add QA captain and k3s group to PR
1. Each time a new RC is cut you must update the KDM PR with the new release information
   1. it can be helpful to add the secondary/backup release captain to your fork so that they can also update the PR if necessary

If a PR already exists, add the new commits to the PR rather than generating a new one.

In some cases you may need to generate two PRs, ask the QA lead.
For example, currently (28 Sep 2022) we generate a PR against branch dev-v2.6 and branch dev-v2.7.