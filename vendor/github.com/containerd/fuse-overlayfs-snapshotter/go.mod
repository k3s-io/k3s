module github.com/containerd/fuse-overlayfs-snapshotter

go 1.16

require (
	github.com/containerd/containerd v1.5.2
	github.com/containerd/continuity v0.1.0
	github.com/coreos/go-systemd/v22 v22.1.0
	github.com/pkg/errors v0.9.1
	github.com/sirupsen/logrus v1.7.0
	google.golang.org/grpc v1.33.2
)

// replace grpc to match https://github.com/containerd/containerd/blob/v1.5.2/go.mod
replace google.golang.org/grpc => google.golang.org/grpc v1.27.1
