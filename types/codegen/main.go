package main

import (
	bindata "github.com/jteeuwen/go-bindata"
	v1 "github.com/rancher/k3s/types/apis/k3s.cattle.io/v1"
	"github.com/rancher/norman/generator"
	"github.com/sirupsen/logrus"
)

var (
	basePackage = "github.com/rancher/k3s/types"
)

func main() {
	bc := &bindata.Config{
		Input: []bindata.InputConfig{
			{
				Path:      "build/data",
				Recursive: true,
			},
		},
		Package:    "data",
		NoCompress: true,
		NoMemCopy:  true,
		NoMetadata: true,
		Output:     "pkg/data/zz_generated_bindata.go",
	}
	if err := bindata.Translate(bc); err != nil {
		logrus.Fatal(err)
	}

	bc = &bindata.Config{
		Input: []bindata.InputConfig{
			{
				Path: "manifests",
			},
		},
		Package:    "deploy",
		NoMetadata: true,
		Prefix:     "manifests/",
		Output:     "pkg/deploy/zz_generated_bindata.go",
	}
	if err := bindata.Translate(bc); err != nil {
		logrus.Fatal(err)
	}

	if err := generator.DefaultGenerate(v1.Schemas, basePackage, false, nil); err != nil {
		logrus.Fatal(err)
	}
}
