---
name: Cut Release
about: Create a new release checklist
title: 'Cut VERSION'
labels: 'kind/release'
assignees: ''

---

**Summary:**
Task covering patch release work.

Dev Complete: RELEASE_DATE (Typically ~1 week prior to upstream release date)

**List of required releases:**
_To release as soon as able for QA:_
- VERSION

_To release once have approval from QA:_
- VERSION (Never release on a Friday unless specified otherwise)

**Prep work:**
- [ ] PM: Dev and QA team to be notified of the incoming releases - add event to team calendar
- [ ] PM: Dev and QA team to be notified of the date we will mark the latest release as stable - add event to team calendar [ONLY APPLICABLE FOR LATEST MINOR RELEASE]
- [ ] QA: Review changes and understand testing efforts
- [ ] Release Captain: Prepare release notes in our private release-notes repo (submit PR for changes taking care to carefully check links and the components, once merged, create the release in GitHub and mark as a draft and check the pre-release box, fill in title, set target release branch, leave tag version blank for now until we are ready to release)
- [ ] QA: Validate and close out all issues in the release milestone.

**Vendor and release work:**
- [ ] Release Captain: Vendor in the new patch version and release rancher/kubernetes
- [ ] Release Captain: Tag and release any necessary RCs for QA to test K3s and KDM on the Rancher side
- [ ] Release Captain: Tag and release when have QA approval

**Post-Release work:**
- [ ] Release Captain: Once release is fully complete (CI is all green and all release artifacts exist), edit the release, uncheck "Pre-release", and save.
- [ ] Release Captain: Prepare PRs as needed to update [KDM](https://github.com/rancher/kontainer-driver-metadata/) in the appropriate dev branches.
- [ ] PM: Close the milestone in GitHub.
