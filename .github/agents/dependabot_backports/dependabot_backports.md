# Monthly GitHub Action SHA Backports

Backport GitHub Actions SHA pin updates from `main` into the three newest `release-1.XX` branches.

## Required outcome

Create at most one pull request per target release branch. Each PR must include all relevant backports for that branch.

## Avaliable tools
- `gh` GitHub CLI for branch, commit, and PR operations.
- `git` for local repository operations.
- `yamllint` for validating yaml files after applying changes.
- `bash` for basic scripting needs. Prefer bash for git operations.
- `python` for advanced scripting needs
- Use any bash and python scripts already created in this folder for assistance.
- Update and reuse any existing scripts in this folder as needed to meet the requirements and constraints of this task.

## Known issues
- Don't lose track of this folder, as it may not exist in older release branches. If it doesn't exist, create it as needed to store any scripts or other files required for the backport process.
- Some commit merges may mess up the spacing of the yaml files. Double check that the yaml is still valid after applying all the commits. 

## Steps

1. Ensure that the local repository is up to date with the latest changes from `main` and all `release-1.XX` branches.
2. Identify all remote branches that match `release-1.XX`.
3. Parse the numeric suffix and select the three highest versions.
4. Read all `.github/workflows/*.{yml,yaml}` files on `main` and extract action references pinned to full SHAs (`uses: owner/repo@<40-hex>`).
5. For each target release branch, extract the same pinned references and compare to `main`.
6. Build the set of action SHA updates that are missing in the release branch.
7. Find the corresponding Dependabot commits on `main` that updated those workflow action SHAs.
   - Prefer commits authored by `dependabot[bot]`.
   - Only include commits that touch workflow files and are relevant to missing action SHA pins.
   - Ensure all commits are signed `-S` to meet DCO requirements.
8. Prepare one update branch per target release branch and apply all relevant updates for that branch.
   - Create a new branch from the target release branch named `dependabot-backports/release-1.XX`. Have it track `origin` and push to it when ready. Example: `git push -u origin dependabot-backports/release-1.34`
   - Prefer cherry-picking the matching Dependabot commits in chronological order.
   - If a cherry-pick does not apply cleanly, manually apply only the workflow `uses:` SHA pin changes needed to match `main`.
9. Open exactly one PR per target release branch using `create-pull-request`.
   - Base branch: target `release-1.XX` branch.
   - Target branch: should follow naming convetion of `dependabot-backports/release-1.XX`.
   - Title: `Backport GitHub Action SHA pin updates from main to <branch>`.
   - Body must include: updated actions, old/new SHAs, and source Dependabot commit links.
10. Skip PR creation for branches that already match `main` for all relevant action SHA pins.
11. If an open PR already exists for the same target branch and purpose, update it instead of creating a duplicate. This will require force rebasing the existing PR branch with the latest changes from 'release-1.XX' and reapplying the necessary commits or changes.

## Constraints

- Only modify workflow action SHA pin references.
- Do not change workflow logic beyond what is required for those SHA pin backports.
- Only ever push and track to `origin` when creating or updating PR branches. Do not push directly to any `release-1.XX` branches.
- Keep PRs focused and minimal.