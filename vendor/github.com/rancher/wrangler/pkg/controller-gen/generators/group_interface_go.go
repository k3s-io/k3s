package generators

import (
	"fmt"
	"io"

	args2 "github.com/rancher/wrangler/pkg/controller-gen/args"
	"k8s.io/gengo/args"
	"k8s.io/gengo/generator"
	"k8s.io/gengo/namer"
)

func GroupInterfaceGo(group string, args *args.GeneratorArgs, customArgs *args2.CustomArgs) generator.Generator {
	return &interfaceGo{
		group:      group,
		args:       args,
		customArgs: customArgs,
		DefaultGen: generator.DefaultGen{
			OptionalName: "interface",
			OptionalBody: []byte(interfaceBody),
		},
	}
}

type interfaceGo struct {
	generator.DefaultGen

	group      string
	args       *args.GeneratorArgs
	customArgs *args2.CustomArgs
}

func (f *interfaceGo) Imports(*generator.Context) []string {
	group := f.customArgs.Options.Groups[f.group]

	packages := []string{
		GenericPackage,
		fmt.Sprintf("clientset \"%s\"", group.ClientSetPackage),
		fmt.Sprintf("informers \"%s/%s\"", group.InformersPackage, groupPackageName(f.group, group.PackageName)),
	}

	for gv := range f.customArgs.TypesByGroup {
		if gv.Group != f.group {
			continue
		}

		packages = append(packages, fmt.Sprintf("%s \"%s/controllers/%s/%s\"", gv.Version, f.customArgs.Package, groupPackageName(gv.Group, ""), gv.Version))
	}

	return packages
}

func (f *interfaceGo) Init(c *generator.Context, w io.Writer) error {
	sw := generator.NewSnippetWriter(w, c, "{{", "}}")
	sw.Do("type Interface interface {\n", nil)
	for gv := range f.customArgs.TypesByGroup {
		if gv.Group != f.group {
			continue
		}

		sw.Do("{{.upperVersion}}() {{.version}}.Interface\n", map[string]interface{}{
			"upperVersion": namer.IC(gv.Version),
			"version":      gv.Version,
		})
	}
	sw.Do("}\n", nil)

	if err := f.DefaultGen.Init(c, w); err != nil {
		return err
	}

	for gv := range f.customArgs.TypesByGroup {
		if gv.Group != f.group {
			continue
		}

		m := map[string]interface{}{
			"upperGroup":   upperLowercase(f.group),
			"upperVersion": namer.IC(gv.Version),
			"version":      gv.Version,
		}
		sw.Do("\nfunc (g *group) {{.upperVersion}}() {{.version}}.Interface {\n", m)
		sw.Do("return {{.version}}.New(g.controllerManager, g.client.{{.upperGroup}}{{.upperVersion}}(), g.informers.{{.upperVersion}}())\n", m)
		sw.Do("}\n", m)
	}

	return sw.Error()
}

var interfaceBody = `
type group struct {
	controllerManager *generic.ControllerManager
	informers         informers.Interface
	client            clientset.Interface
}

// New returns a new Interface.
func New(controllerManager *generic.ControllerManager, informers informers.Interface,
	client clientset.Interface) Interface {
	return &group{
		controllerManager: controllerManager,
		informers:         informers,
		client:            client,
	}
}
`
