package generators

import (
	"strings"

	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/gengo/args"
	"k8s.io/gengo/generator"
)

func Package(arguments *args.GeneratorArgs, name string, generators func(context *generator.Context) []generator.Generator) generator.Package {
	boilerplate, err := arguments.LoadGoBoilerplate()
	runtime.Must(err)

	parts := strings.Split(name, "/")
	return &generator.DefaultPackage{
		PackageName:   groupPath(parts[len(parts)-1]),
		PackagePath:   name,
		HeaderText:    boilerplate,
		GeneratorFunc: generators,
	}
}
