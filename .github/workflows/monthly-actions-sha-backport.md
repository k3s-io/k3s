---
on:
  schedule:
    - cron: "17 4 1 * *"
  workflow_dispatch:

permissions: read-all

tools:
  github:
    toolsets: [default]

safe-outputs:
  create-pull-request:
    max: 3

network: defaults
---

# Monthly GitHub Action SHA Backports

Backport GitHub Actions SHA pin updates from `main` into the three newest `release-1.XX` branches.

## Required outcome

Create at most one pull request per target release branch. Each PR must include all relevant backports for that branch.

## Steps

1. Identify all remote branches that match `release-1.XX`.
2. Parse the numeric suffix and select the three highest versions.
3. Read all `.github/workflows/*.{yml,yaml}` files on `main` and extract action references pinned to full SHAs (`uses: owner/repo@<40-hex>`).
4. For each target release branch, extract the same pinned references and compare to `main`.
5. Build the set of action SHA updates that are missing in the release branch.
6. Find the corresponding Dependabot commits on `main` that updated those workflow action SHAs.
   - Prefer commits authored by `dependabot[bot]`.
   - Only include commits that touch workflow files and are relevant to missing action SHA pins.
7. Prepare one update branch per target release branch and apply all relevant updates for that branch.
   - Prefer cherry-picking the matching Dependabot commits in chronological order.
   - If a cherry-pick does not apply cleanly, manually apply only the workflow `uses:` SHA pin changes needed to match `main`.
8. Open exactly one PR per target release branch using `create-pull-request`.
   - Base branch: target `release-1.XX` branch.
   - Title: `Backport GitHub Action SHA pin updates from main to <branch>`.
   - Body must include: updated actions, old/new SHAs, and source Dependabot commit links.
9. Skip PR creation for branches that already match `main` for all relevant action SHA pins.
10. If an open PR already exists for the same target branch and purpose, update it instead of creating a duplicate.

## Constraints

- Only modify workflow action SHA pin references.
- Do not change workflow logic beyond what is required for those SHA pin backports.
- Keep PRs focused and minimal.
