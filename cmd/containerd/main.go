package main

import (
	"github.com/k3s-io/k3s/pkg/containerd"
	"k8s.io/klog/v2"
)

func main() {
	klog.InitFlags(nil)
	containerd.Main()
}
