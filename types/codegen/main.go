package main

import (
	"github.com/rancher/norman/generator"
	v1 "github.com/rancher/rio/types/apis/k3s.cattle.io/v1"
	"github.com/sirupsen/logrus"
)

var (
	basePackage = "github.com/rancher/rio/types"
)

func main() {
	if err := generator.DefaultGenerate(v1.Schemas, basePackage, false, nil); err != nil {
		logrus.Fatal(err)
	}
}
