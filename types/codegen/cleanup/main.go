package main

import (
	"github.com/rancher/norman/generator/cleanup"
	"github.com/sirupsen/logrus"
)

func main() {
	if err := cleanup.Cleanup("./types"); err != nil {
		logrus.Fatal(err)
	}
}
