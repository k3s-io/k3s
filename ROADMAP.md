Roadmap
---
The k3s project uses [GitHub Milestones](http://github.com/k3s-io/k3s/milestones) to track the progress of changes going into the project.

The k3s release cycle moves in cadence with upstream Kubernetes, with an aim to have new minor releases out within 30 days of upstream .0 releases.  To follow incoming changes, watching the [Backlog](https://github.com/orgs/k3s-io/projects/5) and [Current Development](https://github.com/orgs/k3s-io/projects/6) GitHub Projects is the most up to date way to see what's coming in upcooming releases.

The development of k3s itself happens in the `main` branch, which correlates to the most recent Kubernetes minor release.  These changes are then backported to the active release lines (at this time, `release-[N]`, `release-[N-1]`, and `release-[N-2]`)