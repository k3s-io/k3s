Wrangler [In Development - Does Not Work]
--------
Framework for wrapping clients, informers, listers into a simple
usable controller pattern that promotes some good practices.

More documentation to follow but if you want to see what it
looks like to write a controller with this framework refer to
[main.go](https://github.com/rancher/wrangler/blob/master/sample/main.go) and [controller.go](https://github.com/rancher/wrangler/blob/master/sample/controller.go) in
 the [sample](https://github.com/rancher/wrangler/blob/master/sample).
 
Sample Project
------
The sample project does the same things as the standard Kubernetes [sample controller](https://github.com/kubernetes/sample-controller) but
just using this framework and patterns.

To use the sample clone this project to a proper GOPATH and then

```bash
cd $GOPATH/src/github.com/rancher/wrangler/sample
go generate
go build .
./sample
```

How it works
------------

Most people writing controllers are a bit lost when they go to write a controller as they
find that there is nothing in Kubernetes that is like `type Controller interface` where you
can just do `NewController`.  Instead a controller is really just a pattern of how you use
the generated clientsets, informers, and listers combined with some custom event handlers and
a workqueue.

Wrangler providers a code generator that will generate the clientset, informers, listers and
additionally generate a controller per type.  The interface to the
controller looks as follows

To use the controller all one needs to do is register simple OnChange handlers.  Also in the
interface is access to the client and caches in a simple flat API. refer to
[main.go](https://github.com/rancher/wrangler/blob/master/sample/main.go) and [controller.go](https://github.com/rancher/wrangler/blob/master/sample/controller.go) in
 the [sample project](https://github.com/rancher/wrangler/blob/master/sample) for more complete usage.

```golang
type FooHandler func(string, *v1alpha1.Foo) (*v1alpha1.Foo, error)

type FooController interface {
	Create(*v1alpha1.Foo) (*v1alpha1.Foo, error)
	Update(*v1alpha1.Foo) (*v1alpha1.Foo, error)
	UpdateStatus(*v1alpha1.Foo) (*v1alpha1.Foo, error)
	Delete(namespace, name string, options *metav1.DeleteOptions) error
	DeleteCollection(namespace string, options *metav1.DeleteOptions, listOptions metav1.ListOptions) error
	Get(namespace, name string, options metav1.GetOptions) (*v1alpha1.Foo, error)
	List(namespace string, opts metav1.ListOptions) (*v1alpha1.FooList, error)
	Watch(namespace string, opts metav1.ListOptions) (watch.Interface, error)
	Patch(namespace, name string, pt types.PatchType, data []byte, subresources ...string) (result *v1alpha1.Foo, err error)

	Cache() FooControllerCache

	OnChange(ctx context.Context, name string, sync FooHandler)
	OnRemove(ctx context.Context, name string, sync FooHandler)
	Enqueue(namespace, name string)

	Informer() cache.SharedIndexInformer
	GroupVersionKind() schema.GroupVersionKind
}

type FooControllerCache interface {
	Get(namespace, name string) (*v1alpha1.Foo, error)
	List(namespace string, selector labels.Selector) ([]*v1alpha1.Foo, error)

	AddIndexer(indexName string, indexer FooIndexer)
	GetByIndex(indexName, key string) ([]*v1alpha1.Foo, error)
}

type FooIndexer func(obj *v1alpha1.Foo) ([]string, error)

```
