package generators

import (
	"fmt"
	"io"
	"path/filepath"

	args2 "github.com/rancher/wrangler/pkg/controller-gen/args"
	"k8s.io/gengo/args"
	"k8s.io/gengo/generator"
)

func FactoryGo(group string, args *args.GeneratorArgs, customArgs *args2.CustomArgs) generator.Generator {
	return &factory{
		group:      group,
		args:       args,
		customArgs: customArgs,
		DefaultGen: generator.DefaultGen{
			OptionalName: "factory",
			OptionalBody: []byte(factoryBody),
		},
	}
}

type factory struct {
	generator.DefaultGen

	group      string
	args       *args.GeneratorArgs
	customArgs *args2.CustomArgs
}

func (f *factory) Imports(*generator.Context) []string {
	group := f.customArgs.Options.Groups[f.group]

	return []string{
		"context",
		"time",
		"k8s.io/apimachinery/pkg/runtime/schema",
		"k8s.io/client-go/rest",
		GenericPackage,
		AllSchemes,
		fmt.Sprintf("clientset \"%s\"", group.ClientSetPackage),
		fmt.Sprintf("scheme \"%s\"", filepath.Join(group.ClientSetPackage, "scheme")),
		fmt.Sprintf("informers \"%s\"", group.InformersPackage),
	}
}

func (f *factory) Init(c *generator.Context, w io.Writer) error {
	if err := f.DefaultGen.Init(c, w); err != nil {
		return err
	}

	sw := generator.NewSnippetWriter(w, c, "{{", "}}")
	m := map[string]interface{}{
		"groupName": upperLowercase(f.group),
	}

	sw.Do("func (c *Factory) {{.groupName}}() Interface {\n", m)
	sw.Do("	return New(c.controllerManager, c.informerFactory.{{.groupName}}(), c.clientset)\n", m)
	sw.Do("}\n", m)

	return sw.Error()
}

var factoryBody = `
func init() {
	scheme.AddToScheme(schemes.All)
}

type Factory struct {
	synced            bool
	informerFactory   informers.SharedInformerFactory
	clientset         clientset.Interface
	controllerManager *generic.ControllerManager
	threadiness       map[schema.GroupVersionKind]int
}

func NewFactoryFromConfigOrDie(config *rest.Config) *Factory {
	f, err := NewFactoryFromConfig(config)
	if err != nil {
		panic(err)
	}
	return f
}

func NewFactoryFromConfig(config *rest.Config) (*Factory, error) {
	cs, err := clientset.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	informerFactory := informers.NewSharedInformerFactory(cs, 2*time.Hour)
	return NewFactory(cs, informerFactory), nil
}

func NewFactoryFromConfigWithNamespace(config *rest.Config, namespace string) (*Factory, error) {
	if namespace == "" {
		return NewFactoryFromConfig(config)
	}

	cs, err := clientset.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	informerFactory := informers.NewSharedInformerFactoryWithOptions(cs, 2*time.Hour, informers.WithNamespace(namespace))
	return NewFactory(cs, informerFactory), nil
}


func NewFactory(clientset clientset.Interface, informerFactory informers.SharedInformerFactory) *Factory {
	return &Factory{
		threadiness:       map[schema.GroupVersionKind]int{},
		controllerManager: &generic.ControllerManager{},
		clientset:         clientset,
		informerFactory:   informerFactory,
	}
}

func (c *Factory) SetThreadiness(gvk schema.GroupVersionKind, threadiness int) {
	c.threadiness[gvk] = threadiness
}

func (c *Factory) Sync(ctx context.Context) error {
	c.informerFactory.Start(ctx.Done())
	c.informerFactory.WaitForCacheSync(ctx.Done())
	return nil
}

func (c *Factory) Start(ctx context.Context, defaultThreadiness int) error {
	if err := c.Sync(ctx); err != nil {
		return err
	}

	return c.controllerManager.Start(ctx, defaultThreadiness, c.threadiness)
}

`
