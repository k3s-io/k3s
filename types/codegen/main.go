package main

import (
	bindata "github.com/jteeuwen/go-bindata"
	v1 "github.com/rancher/k3s/types/apis/k3s.cattle.io/v1"
	"github.com/rancher/norman/generator"
	"github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
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
				Path:      "build/static",
				Recursive: true,
			},
		},
		Package:    "static",
		NoMetadata: true,
		Prefix:     "build/static/",
		Output:     "pkg/static/zz_generated_bindata.go",
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

	if err := generator.ControllersForForeignTypes(basePackage, corev1.SchemeGroupVersion, []interface{}{
		corev1.ServiceAccount{},
		corev1.Endpoints{},
		corev1.Service{},
		corev1.Pod{},
		corev1.ConfigMap{},
	}, []interface{}{
		corev1.Node{},
	}); err != nil {
		logrus.Fatal(err)
	}

	if err := generator.ControllersForForeignTypes(basePackage, appsv1.SchemeGroupVersion, []interface{}{
		appsv1.Deployment{},
	}, nil); err != nil {
		logrus.Fatal(err)
	}

	if err := generator.ControllersForForeignTypes(basePackage, batchv1.SchemeGroupVersion, []interface{}{
		batchv1.Job{},
	}, nil); err != nil {
		logrus.Fatal(err)
	}

	if err := generator.ControllersForForeignTypes(basePackage, rbacv1.SchemeGroupVersion, nil, []interface{}{
		rbacv1.ClusterRoleBinding{},
	}); err != nil {
		logrus.Fatal(err)
	}
}
