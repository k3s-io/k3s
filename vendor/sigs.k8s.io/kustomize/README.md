# kustomize

`kustomize` lets you customize raw, template-free YAML
files for multiple purposes, leaving the original YAML
untouched and usable as is.

`kustomize` targets kubernetes; it understands and can
patch [kubernetes style] API objects.  It's like
[`make`], in that what it does is declared in a file,
and it's like [`sed`], in that it emits editted text.

This tool is sponsored by [sig-cli] ([KEP]), and
inspired by [DAM].


[![Build Status](https://travis-ci.org/kubernetes-sigs/kustomize.svg?branch=master)](https://travis-ci.org/kubernetes-sigs/kustomize)
[![Go Report Card](https://goreportcard.com/badge/github.com/kubernetes-sigs/kustomize)](https://goreportcard.com/report/github.com/kubernetes-sigs/kustomize)

**Installation**: Download a binary from the [release
page], or see these [install] notes. Then try one of
the tested [examples].

## Usage


### 1) Make a [kustomization] file

In some directory containing your YAML [resource]
files (deployments, services, configmaps, etc.), create a
[kustomization] file.

This file should declare those resources, and any
customization to apply to them, e.g. _add a common
label_.

![base image][imageBase]

File structure:

> ```
> ~/someApp
> ├── deployment.yaml
> ├── kustomization.yaml
> └── service.yaml
> ```

The resources in this directory could be a fork of
someone else's configuration.  If so, you can easily
rebase from the source material to capture
improvements, because you don't modify the resources
directly.

Generate customized YAML with:

```
kustomize build ~/someApp
```

The YAML can be directly [applied] to a cluster:

> ```
> kustomize build ~/someApp | kubectl apply -f -
> ```


### 2) Create [variants] using [overlays]

Manage traditional [variants] of a configuration - like
_development_, _staging_ and _production_ - using
[overlays] that modify a common [base].

![overlay image][imageOverlay]

File structure:
> ```
> ~/someApp
> ├── base
> │   ├── deployment.yaml
> │   ├── kustomization.yaml
> │   └── service.yaml
> └── overlays
>     ├── development
>     │   ├── cpu_count.yaml
>     │   ├── kustomization.yaml
>     │   └── replica_count.yaml
>     └── production
>         ├── cpu_count.yaml
>         ├── kustomization.yaml
>         └── replica_count.yaml
> ```

Take the work from step (1) above, move it into a
`someApp` subdirectory called `base`, then
place overlays in a sibling directory.

An overlay is just another kustomization, refering to
the base, and referring to patches to apply to that
base.

This arrangement makes it easy to manage your
configuration with `git`.  The base could have files
from an upstream repository managed by someone else.
The overlays could be in a repository you own.
Arranging the repo clones as siblings on disk avoids
the need for git submodules (though that works fine, if
you are a submodule fan).

Generate YAML with

```sh
kustomize build ~/someApp/overlays/production
```

The YAML can be directly [applied] to a cluster:

> ```sh
> kustomize build ~/someApp/overlays/production | kubectl apply -f -
> ```

## Community

### Filing bug reports


##### A good report specifies

 * the output of `kustomize version`,
 * the input (the content of `kustomization.yaml`
   and any files it refers to),
 * the expected YAML output.

##### A _great_ report is a bug reproduction test
 
Kustomize has a simple test harness in the
[target package] for specifying a kustomization's
input and the expected output.
See this [example of a target test].
 
The pattern is
 * call `NewKustTestHarness`
 * specify kustomization input data (resources,
   patches, etc.) as inline strings,
 * call `makeKustTarget().MakeCustomizedResMap()`
 * compare the actual output to expected output

In a bug reproduction test, the expected output string
initially contains the _wrong_ (unexpected) output,
thus unambiguously reproducing the bug.

Nearby comments should explain what the output
_should_ be, and have a TODO pointing to the related
issue.

The person who fixes the bug then has a clear
bug reproduction and a test to modify when
the bug is fixed.

The bug reporter can then see the bug was fixed,
and has permanent regression coverage to prevent
its reintroduction.
 
### Feature requests

Feature requests are welcome.

Before working on an implementation, please
 * Read the [eschewed feature list].
 * File an issue describing
   how the new feature would behave
   and label it [kind/feature].

### Other communication channels

- [Slack]
- [Mailing List]
- General kubernetes [community page]

### Code of conduct

Participation in the Kubernetes community
is governed by the [Kubernetes Code of Conduct].

[`make`]: https://www.gnu.org/software/make
[`sed`]: https://www.gnu.org/software/sed
[DAM]: docs/glossary.md#declarative-application-management
[KEP]: https://github.com/kubernetes/enhancements/blob/master/keps/sig-cli/0008-kustomize.md
[Kubernetes Code of Conduct]: code-of-conduct.md
[Mailing List]: https://groups.google.com/forum/#!forum/kubernetes-sig-cli
[Slack]: https://kubernetes.slack.com/messages/sig-cli
[applied]: docs/glossary.md#apply
[base]: docs/glossary.md#base
[community page]: http://kubernetes.io/community/
[declarative configuration]: docs/glossary.md#declarative-application-management
[eschewed feature list]: docs/eschewedFeatures.md
[example of a target test]: https://github.com/kubernetes-sigs/kustomize/blob/master/pkg/target/baseandoverlaysmall_test.go
[examples]: examples/README.md
[imageBase]: docs/base.jpg
[imageOverlay]: docs/overlay.jpg
[install]: docs/INSTALL.md
[kind/feature]: https://github.com/kubernetes-sigs/kustomize/labels/kind%2Ffeature
[kubernetes style]: docs/glossary.md#kubernetes-style-object
[kustomization]: docs/glossary.md#kustomization
[overlay]: docs/glossary.md#overlay
[overlays]: docs/glossary.md#overlay
[release page]: https://github.com/kubernetes-sigs/kustomize/releases
[resource]: docs/glossary.md#resource
[resources]: docs/glossary.md#resource
[sig-cli]: https://github.com/kubernetes/community/blob/master/sig-cli/README.md
[target package]: https://github.com/kubernetes-sigs/kustomize/tree/master/pkg/target
[variant]: docs/glossary.md#variant
[variants]: docs/glossary.md#variant
[workflows]: docs/workflows.md
