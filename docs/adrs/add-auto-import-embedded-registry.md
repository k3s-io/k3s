# Easy way for auto adding images to k3s

Date: 2024-10-2

## Status

Proposed

## Context

Since the feature for embedded registry, the users appeared with a question about having to manually import images, specially in tarball environments
As a result, there is a need for a folder to be created, where every image there will be watched by a controller (a child process that will run when the embedded registry is created) for changes or new images, this new images or new changes will be added to the node registry, meaning that other nodes will have access to the image.

## Decision

- Not decided yet

## Consequences

Good:
- Better use of the embedded registry