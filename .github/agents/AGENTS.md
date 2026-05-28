---
name: K3s Local Agentic Workflows Index
description: Use when: browsing or selecting local k3s agentic workflows from this folder. Catalog of workflow folders and contract for cross-tool compatibility.
type: index
user-invocable: false
---

# Local Agentic Workflows

## Overview

This directory and its subdirections contain custom agentic workflows used in the maintanence of the k3s repository. Each workflow is defined in a folder named after the workflow and includes a markdown file that describes the workflow and its requirements. The folder may also include any scripts used to complete the workflow, often from past agentic runs.

Each folder serves as the input to be passed to a local agentic tool, be that VSCode copilot or Claude Code, Opencode etc. This is a balanced approach to agentic workflows that allows for rapid, automated work, while still ensuring that all work is tied to a single developer and their personal fork, not some generic "Copilot" dev. It is the developers responsibility to ensure that the workflow is completed correctly and that all changes are properly reviewed and merged. These workflows should always result in a pull request or code change that is reviewed and merged by the k3s-io maintainers, never direct commits to main or release branches.

## Workflow Folder Contract

Each workflow folder should contain:
- Required: one workflow markdown file, usually named after the folder.
- Optional: a scripts folder for helper scripts reused across runs.
- Optional: readme or template artifacts for maintainers.

Each workflow markdown file should include:
- YAML frontmatter with: name, description, type, tools, output, user-invocable.
- Section headers in this order: Overview, Required Outcome, Available Tools, Known Issues, Steps, Constraints, Success Criteria.

## Workflows

- dependabot_backports: backport GitHub Action SHA pin updates from main into the newest release branches.
