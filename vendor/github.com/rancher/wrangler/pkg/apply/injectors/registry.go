package injectors

import "k8s.io/apimachinery/pkg/runtime"

var (
	injectors = map[string]ConfigInjector{}
	order     []string
)

type ConfigInjector func(config []runtime.Object) ([]runtime.Object, error)

func Register(name string, injector ConfigInjector) {
	if _, ok := injectors[name]; !ok {
		order = append(order, name)
	}
	injectors[name] = injector
}

func Get(name string) ConfigInjector {
	return injectors[name]
}
