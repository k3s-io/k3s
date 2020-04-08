package generic

import (
	"github.com/rancher/wrangler/pkg/apply"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type GeneratingHandlerOptions struct {
	AllowCrossNamespace bool
	AllowClusterScoped  bool
	NoOwnerReference    bool
	DynamicLookup       bool
}

func ConfigureApplyForObject(apply apply.Apply, obj metav1.Object, opts *GeneratingHandlerOptions) apply.Apply {
	if opts == nil {
		opts = &GeneratingHandlerOptions{}
	}

	if opts.DynamicLookup {
		apply = apply.WithDynamicLookup()
	}

	if opts.NoOwnerReference {
		apply = apply.WithSetOwnerReference(true, false)
	}

	if opts.AllowCrossNamespace && !opts.AllowClusterScoped {
		apply = apply.
			WithDefaultNamespace(obj.GetNamespace()).
			WithListerNamespace(obj.GetNamespace())
	}

	if !opts.AllowClusterScoped {
		apply = apply.WithRestrictClusterScoped()
	}

	return apply
}
