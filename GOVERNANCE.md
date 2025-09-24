# K3s Project Governance

This governance explains how the K3s project is run. As such that's a living document that can be updated at anytime.

- [Values](#values)  
- [Maintainers](#maintainers)  
- [Becoming a Maintainer](#becoming-a-maintainer)  
- [Meetings](#meetings)  
- [CNCF Resources](#cncf-resources)  
- [Code of Conduct Enforcement](#code-of-conduct)  
- [Security Response Team](#security-response-team)  
- [Voting](#voting)  
- [Modifications](#modifying-this-charter)

## Values

K3s and its leadership embrace the following values:

* Openness: Communication and decision-making happens in the open and is discoverable for future reference. As much as possible, all discussions and work take place in public forums and open repositories.  
    
* Fairness: All stakeholders have the opportunity to provide feedback and submit contributions, which will be considered on their merits.  
    
* Community over Product or Company: Sustaining and growing our community takes priority over shipping code or sponsors' organizational goals.  Each contributor participates in the project as an individual.  
    
* Inclusivity: We innovate through different perspectives and skill sets, which can only be accomplished in a welcoming and respectful environment.  
    
* Participation: Responsibilities within the project are earned through participation, and there is a clear path up the contributor ladder into leadership positions.

## Community Roles

* **Users**: Members that engage with the K3s community via any medium (Slack, GitHub, mailing lists, etc.).

* **Contributors**: Regular contributions to projects (documentation, code reviews, responding to issues, participation in proposal discussions, contributing code, etc.). Contributors are potential Maintainers.

* **Reviewers**: Review contributions from other members.

* **Maintainers**: The K3s project leaders. They are responsible for the overall health and direction of the project; final reviewers of PRs and responsible for releases. Some Maintainers are responsible for one or more components within a project, acting as technical leads for that component. Maintainers are expected to contribute code and documentation, review PRs including ensuring quality of code, triage issues, proactively fix bugs, and perform maintenance tasks for these components.

## Maintainers

K3s Maintainers have write access to the [project GitHub repository](https://github.com/k3s-io/k3s/). They can merge their own patches(after approval process) or patches from others. The current Maintainers can be found in [MAINTAINERS.md](https://github.com/k3s-io/k3s/blob/main/MAINTAINERS).  Maintainers collectively manage the project's resources and contributors.

This privilege carries specific responsibilities: Maintainers are people who care about the K3s project and want to help it grow and improve. A Maintainer is not just someone who can make changes, but someone who has demonstrated their ability to collaborate with the team, get the most knowledgeable people to review code and docs, contribute high-quality code, and follow through to fix issues (in code or tests).

A Maintainer is a contributor to the project's success and a citizen helping the project succeed.

The collective team of all Maintainers is known as the Maintainer Council, which is the governing body for the project.

### 

### 

### Becoming a Maintainer

Anyone is eligible to become a Maintainer, you need to demonstrate a few or more of the following:

* demonstrate availability and capability to meet the Maintainer expectations above  
* commitment to the project:  
  * participate in discussions, community meeting, contributions, code and documentation reviews for period 6 months or more,  
  * perform reviews for 15 non-trivial pull requests,  
  * contribute 30 non-trivial pull requests and have them merged,  
* ability to write quality code and/or documentation,  
* ability to collaborate with the team,  
* understanding of how the team works (policies, processes for testing and code review, etc),  
* understanding of the project's code base and coding and documentation style.

A new Maintainer must be proposed by an existing Maintainer by sending a message to the [developer mailing list](mailto:k3s-maintainers@lists.cncf.io) and opening PR in [MAINTAINERS](https://github.com/k3s-io/k3s/blob/main/MAINTAINERS). A [supermajority](#Supermajority) vote of existing Maintainers approves the application.  Maintainer nominations will be evaluated without prejudice to employer or demographics.

Maintainers who are selected will be granted the necessary GitHub rights, and invited to the [private Maintainer mailing list](mailto:k3s-maintainers@lists.cncf.io).

Maintainer’s responsibilities:

* **Code Quality & Reviews**: Review and merge pull requests, ensure adherence to project standards, and manage the codebase  
* **Issue Management**: Triage incoming issues, provide guidance, and close or categorize as needed  
* **Releases**: Oversee versioning, changelogs, and publication of new releases  
* **Community Stewardship**: Foster a welcoming and inclusive environment, respond to contributor questions, and enforce the Code of Conduct  
* **Documentation**: Maintain clear user and contributor documentation  
* **Project Direction**: Set and communicate project goals, prioritize features and fixes  
* **Security & Dependencies**: Keep dependencies secure and up to date, monitor for vulnerabilities  
* **Onboarding & Mentorship**: Support new contributors and encourage diverse forms of contribution  
* **Advocacy & Outreach**: Promote the project externally and collaborate with other communities when appropriate

### Removing a Maintainer

Maintainers may resign at any time if they feel that they will not be able to continue fulfilling their project duties.

Maintainers may also be removed after being inactive, failure to fulfill their Maintainer responsibilities, violating the Code of Conduct, or other reasons. Inactivity is defined as a period of very low or no activity in the project for a year or more, with no definite schedule to return to full Maintainer activity.

A Maintainer may be removed at any time by a [supermajority](#Supermajority) vote of the remaining Maintainers.

Depending on the reason for removal, a Maintainer may be converted to Emeritus status.  Emeritus Maintainers will still be consulted on some project matters, and can be rapidly returned   
to Maintainer status if their availability changes.

## Reviewer  
Reviewers are able to review code for quality and correctness on some part of a subproject. They are knowledgeable about both the codebase and software engineering principles.

**Requirements**

* Knowledgeable about the codebase  
* Sponsored by a Maintainer  
* New reviewer must be nominated by an existing Maintainer or reviewer or self-nominated and must be elected by a [supermajority](#Supermajority) of existing Maintainers

**Responsibilities and privileges**

* Code reviewer status may be a precondition to accepting large code contributions  
* Responsible for project quality control via code reviews  
* Focus on code quality and correctness, including testing and factoring  
* May also review for more holistic issues, but not a requirement  
* Expected to be responsive to review requests  
* Assigned PRs to review related to subproject of expertise  
* Assigned test bugs related to subproject of expertise  
  


## Community Ladder

User \-\> Contributor \-\> Reviewer \-\> Maintainer

## Supermajority

A **supermajority** is defined as two-thirds (2/3) of active Maintainers. A supermajority is calculated based on the number of votes cast, excluding abstentions.

Maintainers may vote "agree / yes / +1", "disagree / no / -1", or "abstain". An "abstain" vote does not count toward the total used to calculate the supermajority — it is equivalent to not voting.

A vote passes when at least two-thirds of the non-abstaining votes are in favor within the voting period.

Votes must be cast within a defined voting period (e.g., 7 calendar days). If a quorum (more than 50% of active Maintainers) is not met during this period, the vote is considered invalid and may be rescheduled.

Failure to vote on major decisions will be considered a sign of inactivity and may indicate that a Maintainer is not fulfilling their responsibilities. This behavior will be taken into account during periodic reviews of Maintainer status.

Examples:
| Votes Cast | Yes Votes | No Votes | Required Yes Votes (≥2/3) | Result               |
|------------|-----------|----------|----------------------------|----------------------|
| 9          | 6         | 3        | 6                          | ✅ Passes (6 = 2/3)   |
| 9          | 5         | 4        | 6                          | ❌ Fails (5 < 2/3)    |
| 6          | 4         | 2        | 4                          | ✅ Passes (4 = 2/3)   |
| 6          | 3         | 3        | 4                          | ❌ Fails (3 < 2/3)    |
| 12         | 8         | 4        | 8                          | ✅ Passes (8 = 2/3)   |
| 12         | 7         | 5        | 8                          | ❌ Fails (7 < 2/3)    |
 

## Simple majority

A [**Simple majority**](#simple-majority) is defined as: more than half of the votes cast in a decision-making process.

Examples:
| Votes Cast | Yes Votes | No Votes | Result               |
|------------|-----------|----------|----------------------|
| 10         | 6         | 4        | ✅ Passes (6 > 5)     |
| 10         | 5         | 5        | ❌ Fails (not > 50%)  |
| 7          | 4         | 3        | ✅ Passes (4 > 3)     |
| 7          | 3         | 3        | ❌ Fails (tie; not >) |


## Voting and Decision Making

While most business in K3s is conducted by "[lazy consensus](https://community.apache.org/committers/lazyConsensus.html)", periodically the Maintainers may need to vote on specific actions or changes. A vote can be taken on [the developer mailing list](mailto:cncf-k3s-dev@lists.cncf.io) or [the private Maintainer mailing list](mailto:cncf-k3s-maintainers@lists.cncf.io) for security or conduct matters.  
Votes may also be taken at [the community meeting](https://k3s.io/community/#community-meetings).  Any Maintainer may demand a vote be taken.

Most votes require a [simple majority](#simple-majority) of all Maintainers to succeed, except where otherwise noted.  [Supermajority](#Supermajority) votes mean at least two-thirds of all existing Maintainers.

Ideally, all project decisions are resolved by consensus. If impossible, any Maintainer may call a vote. Unless otherwise specified in this document, any vote will be decided by a [supermajority](#Supermajority) of Maintainers.

In case of situation with not enough participation from maintainer for **non** critical decision we can lower the supermajority to [**simple majority**](#simple-majority).

For any **critital** decisions [CNCF TOC](https://www.cncf.io/people/technical-oversight-committee/) should be consulted for approvals and moving forward.

## Voting requirements:

* Adding a Maintainer: [Supermajority](#Supermajority)

* Removing a Maintainer:  [Supermajority](#Supermajority)

* Requesting CNCF resources: [Simple majority](#simple-majority)

* Charter and Governance: [Supermajority](#Supermajority)

If a vote does not meet quorum (e.g., fewer than 50% of Maintainers vote), the vote may be postponed or escalated to a follow-up meeting.

## Proposal Process(ADRs)

One of the most important aspects in any open source community is the concept of proposals. Large changes to the codebase and/or new features should be preceded by a proposal in our repository [ADRs](https://github.com/k3s-io/k3s/tree/main/docs/adrs). This process allows for all members of the community to weigh in on the concept (including the technical details), share their comments and ideas, and offer to help. It also ensures that members are not duplicating work or inadvertently stepping on toes by making large conflicting changes.

The project roadmap is defined by accepted proposals.

Proposals should cover the high-level objectives, use cases, and technical recommendations on how to implement. In general, the community member(s) interested in implementing the proposal should be either deeply engaged in the proposal process or be an author of the proposal.

The proposal should be documented as a separate markdown file pushed to the [ADRs directory](https://github.com/k3s-io/k3s/tree/main/docs/adrs) in the k3s repository via PR. The name of the file should follow the name pattern `<short meaningful words joined by '-'>.md`, e.g: `clear-old-tags-with-policies.md`.

Use the [Proposal Template](https://github.com/k3s-io/k3s/tree/main/docs/adrs/template.md) as a starting point.(need to open PR for template)

### Proposal Lifecycle

The proposal PR can be marked with different status labels to represent the status of the proposal:

* Proposed: Proposal is just created.  
* Reviewing: Proposal is under review and discussion.  
* Accepted: Proposal is reviewed and accepted (either by consensus or vote).  
* Rejected: Proposal is reviewed and rejected (either by consensus or vote).

 A proposal may only be accepted and merged after receiving approval from at least two maintainers who are not the original author of the proposal.

### Proposal Threshold
The need for a proposal (in the form of an ADR or design document) is determined primarily by scope. Pull requests that introduce major changes — such as architectural overhauls, system-wide patterns, or significant new features — may prompt maintainers to request a proposal for further discussion and alignment.

Maintainers may comment on a PR with a request to "submit an ADR" when a change is deemed too substantial for review in isolation.

Smaller, self-contained changes (e.g., bug fixes, minor enhancements, or localized refactors) typically do not require a proposal and can proceed through the standard PR workflow.


## Meetings

Time zones permitting, Maintainers are expected to participate in the public developer meeting, which occurs [Community meetings](https://k3s.io/community/\#community-meetings)

Maintainers will also have closed meetings in order to discuss security reports or [Code of Conduct](https://github.com/k3s-io/k3s/blob/main/CODE_OF_CONDUCT.md) violations.  Such meetings should be scheduled by any Maintainer on receipt of a security issue or CoC report.  All current Maintainers must be invited to such closed meetings, except for any Maintainer who is accused of a CoC violation.

## CNCF Resources

Any Maintainer may suggest a request for CNCF resources, either in the [mailing list](k3s-Maintainers@lists.cncf.io), or during a meeting.  A [Simple majority](#simple-majority) of Maintainers approves the request.  The Maintainers may also choose to delegate working with the CNCF to non-Maintainer community members, who will then be added to the [CNCF's Maintainer List](https://github.com/cncf/foundation/blob/main/project-maintainers.csv) for that purpose.

## Code of Conduct

[Code of Conduct](https://github.com/k3s-io/k3s/blob/main/CODE_OF_CONDUCT.md) violations by community members will be discussed and resolved on the [private Maintainer mailing list](https://lists.cncf.io/g/cncf-k3s-maintainers).  If a Maintainer is directly involved in the report, the Maintainers will instead designate two Maintainers to work with the CNCF Code of Conduct Committee in resolving it.

## Security Response Team

The Maintainers will appoint a Security Response Team to handle security reports. This committee may simply consist of the Maintainer Council themselves.  If this responsibility is delegated, the Maintainers will appoint a team of at least two contributors to handle it.  The Maintainers will review who is assigned to this at least once a year.

The Security Response Team is responsible for handling all reports of security holes and breaches according to the [security policy](https://github.com/k3s-io/k3s?tab=security-ov-file\#readme).

## Modifying this Charter

Changes to this Governance and its supporting documents may be approved by a [supermajority](#Supermajority) vote of the Maintainers.

## 

## 

## **Thanks**

Many thanks in advance to everyone who contributes their time and effort to making K3s both a successful project as well as a successful community. The strength of our software shines in the strengths of each individual community member. Thank YOU\!

Some content in this document was built(inspired) upon the work in the [Kubernetes](https://github.com/kubernetes/community), [Linkerd](https://github.com/linkerd/linkerd2/blob/main/GOVERNANCE.md), [Helm](https://github.com/helm/community/blob/main/governance), [Harbor](https://github.com/goharbor/community/blob/main/GOVERNANCE.md), [Contour](https://github.com/projectcontour/community/blob/main/GOVERNANCE.md) Communities\! KUDOs to all of them\!  