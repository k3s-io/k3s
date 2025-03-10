package main

import (
	"os"

	_ "github.com/k3s-io/api/pkg/generated/controllers/k3s.cattle.io/v1"
	k3scrd "github.com/k3s-io/k3s/pkg/crd"
	"github.com/rancher/wrangler/pkg/crd"
)

func main() {
	crd.Print(os.Stdout, k3scrd.List())
}
