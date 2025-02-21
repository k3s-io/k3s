package main

import (
	"os"

	k3scrd "github.com/k3s-io/k3s/pkg/crd"
	_ "github.com/k3s-io/api/pkg/generated/controllers/k3s.cattle.io/v1"
	"github.com/rancher/wrangler/v3/pkg/crd"
)

func main() {
	crd.Print(os.Stdout, k3scrd.List())
}
