package main

import (
	bindata "github.com/jteeuwen/go-bindata"
	v1 "github.com/rancher/k3s/types/apis/k3s.cattle.io/v1"
	"github.com/rancher/norman/generator"
	"github.com/sirupsen/logrus"
	v13 "k8s.io/api/apps/v1"
	v12 "k8s.io/api/core/v1"
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

	bc = &bindata.Config{
		Input: []bindata.InputConfig{
			{
				Path: "vendor/k8s.io/kubernetes/openapi.json",
			},
			{
				Path: "vendor/k8s.io/kubernetes/openapi.pb",
			},
		},
		Package:    "openapi",
		NoMetadata: true,
		Prefix:     "vendor/k8s.io/kubernetes/",
		Output:     "pkg/openapi/zz_generated_bindata.go",
	}
	if err := bindata.Translate(bc); err != nil {
		logrus.Fatal(err)
	}

	if err := generator.DefaultGenerate(v1.Schemas, basePackage, false, nil); err != nil {
		logrus.Fatal(err)
	}

	if err := generator.ControllersForForeignTypes(basePackage, v12.SchemeGroupVersion, []interface{}{
		v12.Service{},
		v12.Pod{},
	}, []interface{}{
		v12.Node{},
	}); err != nil {
		logrus.Fatal(err)
	}

	if err := generator.ControllersForForeignTypes(basePackage, v13.SchemeGroupVersion, []interface{}{
		v13.Deployment{},
	}, nil); err != nil {
		logrus.Fatal(err)
	}
}
