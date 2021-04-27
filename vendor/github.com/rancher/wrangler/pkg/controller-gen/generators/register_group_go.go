package generators

import (
	"fmt"

	args2 "github.com/rancher/wrangler/pkg/controller-gen/args"
	"k8s.io/gengo/args"
	"k8s.io/gengo/generator"
)

func RegisterGroupGo(group string, args *args.GeneratorArgs, customArgs *args2.CustomArgs) generator.Generator {
	return &registerGroupGo{
		group:      group,
		args:       args,
		customArgs: customArgs,
		DefaultGen: generator.DefaultGen{
			OptionalName: "zz_generated_register",
		},
	}
}

type registerGroupGo struct {
	generator.DefaultGen

	group      string
	args       *args.GeneratorArgs
	customArgs *args2.CustomArgs
}

func (f *registerGroupGo) PackageConsts(*generator.Context) []string {
	return []string{
		fmt.Sprintf("GroupName = \"%s\"", f.group),
	}
}
