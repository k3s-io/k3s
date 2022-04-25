package main

import (
	"github.com/k3s-io/k3s/pkg/containerd"
	"k8s.io/klog"
)

func main() {
	klog.InitFlags(nil)
	containerd.Main()
}
