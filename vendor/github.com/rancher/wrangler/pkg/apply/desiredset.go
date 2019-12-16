package apply

import (
	"github.com/rancher/wrangler/pkg/apply/injectors"
	"github.com/rancher/wrangler/pkg/merr"
	"github.com/rancher/wrangler/pkg/objectset"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/cache"
)

type desiredSet struct {
	a                        *apply
	defaultNamespace         string
	listerNamespace          string
	setOwnerReference        bool
	ownerReferenceController bool
	ownerReferenceBlock      bool
	strictCaching            bool
	restrictClusterScoped    bool
	pruneTypes               map[schema.GroupVersionKind]cache.SharedIndexInformer
	patchers                 map[schema.GroupVersionKind]Patcher
	reconcilers              map[schema.GroupVersionKind]Reconciler
	remove                   bool
	noDelete                 bool
	setID                    string
	objs                     *objectset.ObjectSet
	codeVersion              string
	owner                    runtime.Object
	injectors                []injectors.ConfigInjector
	ratelimitingQps          float32
	injectorNames            []string
	errs                     []error
}

func (o *desiredSet) err(err error) error {
	o.errs = append(o.errs, err)
	return o.Err()
}

func (o desiredSet) Err() error {
	return merr.NewErrors(append(o.errs, o.objs.Err())...)
}

func (o desiredSet) Apply(set *objectset.ObjectSet) error {
	if set == nil {
		set = objectset.NewObjectSet()
	}
	o.objs = set
	return o.apply()
}

func (o desiredSet) ApplyObjects(objs ...runtime.Object) error {
	os := objectset.NewObjectSet()
	os.Add(objs...)
	return o.Apply(os)
}

// WithGVK uses a known listing of existing gvks to modify the the prune types to allow for deletion of objects
func (o desiredSet) WithGVK(gvks ...schema.GroupVersionKind) Apply {
	pruneTypes := make(map[schema.GroupVersionKind]cache.SharedIndexInformer, len(gvks))
	for k, v := range o.pruneTypes {
		pruneTypes[k] = v
	}
	for _, gvk := range gvks {
		pruneTypes[gvk] = nil
	}
	o.pruneTypes = pruneTypes
	return o
}

func (o desiredSet) WithSetID(id string) Apply {
	o.setID = id
	return o
}

func (o desiredSet) WithOwner(obj runtime.Object) Apply {
	o.owner = obj
	return o
}

func (o desiredSet) WithSetOwnerReference(controller, block bool) Apply {
	o.setOwnerReference = true
	o.ownerReferenceController = controller
	o.ownerReferenceBlock = block
	return o
}

func (o desiredSet) WithInjector(injs ...injectors.ConfigInjector) Apply {
	o.injectors = append(o.injectors, injs...)
	return o
}

func (o desiredSet) WithInjectorName(injs ...string) Apply {
	o.injectorNames = append(o.injectorNames, injs...)
	return o
}

func (o desiredSet) WithCacheTypes(igs ...InformerGetter) Apply {
	pruneTypes := make(map[schema.GroupVersionKind]cache.SharedIndexInformer, len(igs))
	for k, v := range o.pruneTypes {
		pruneTypes[k] = v
	}

	for _, ig := range igs {
		pruneTypes[ig.GroupVersionKind()] = ig.Informer()
	}

	o.pruneTypes = pruneTypes
	return o
}

func (o desiredSet) WithPatcher(gvk schema.GroupVersionKind, patcher Patcher) Apply {
	patchers := map[schema.GroupVersionKind]Patcher{}
	for k, v := range o.patchers {
		patchers[k] = v
	}
	patchers[gvk] = patcher
	o.patchers = patchers
	return o
}

func (o desiredSet) WithReconciler(gvk schema.GroupVersionKind, reconciler Reconciler) Apply {
	reconcilers := map[schema.GroupVersionKind]Reconciler{}
	for k, v := range o.reconcilers {
		reconcilers[k] = v
	}
	reconcilers[gvk] = reconciler
	o.reconcilers = reconcilers
	return o
}

func (o desiredSet) WithStrictCaching() Apply {
	o.strictCaching = true
	return o
}
func (o desiredSet) WithDynamicLookup() Apply {
	o.strictCaching = false
	return o
}

func (o desiredSet) WithRestrictClusterScoped() Apply {
	o.restrictClusterScoped = true
	return o
}

func (o desiredSet) WithDefaultNamespace(ns string) Apply {
	o.defaultNamespace = ns
	return o
}

func (o desiredSet) WithListerNamespace(ns string) Apply {
	o.listerNamespace = ns
	return o
}

func (o desiredSet) WithRateLimiting(ratelimitingQps float32) Apply {
	o.ratelimitingQps = ratelimitingQps
	return o
}

func (o desiredSet) WithNoDelete() Apply {
	o.noDelete = true
	return o
}
