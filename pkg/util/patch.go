package util

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
)

// controllerPatcher matches the Patch function exposed by rancher/wrangler Controllers
type controllerPatcher[T runtime.Object] interface {
	Patch(name string, pt types.PatchType, data []byte, subresources ...string) (T, error)
}

// clientPatcher matches the Patch function exposed by k8s.io/client-go Clients
type clientPatcher[T runtime.Object] interface {
	Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions, subresources ...string) (T, error)
}

// Patcher wraps the Patch functions provided by either wrangler or client-go
type Patcher[T runtime.Object] struct {
	impl any
}

// NewPatcher wraps the provided controller or client for use as a generic patcher
// note that the patcher is not validated for use until `Patch` is called
func NewPatcher[T runtime.Object](patcher any) *Patcher[T] {
	return &Patcher[T]{
		impl: patcher,
	}
}

// Patch applies the provided PatchList to the specified resource
func (p *Patcher[T]) Patch(ctx context.Context, pl *PatchList, name string, subresources ...string) (T, error) {
	var t T
	if pl == nil {
		pl = NewPatchList()
	}
	b, err := json.Marshal(pl.ops)
	if err != nil {
		return t, err
	}
	if patch, ok := p.impl.(clientPatcher[T]); ok {
		return patch.Patch(ctx, name, types.JSONPatchType, b, metav1.PatchOptions{}, subresources...)
	}
	if patch, ok := p.impl.(controllerPatcher[T]); ok {
		return patch.Patch(name, types.JSONPatchType, b, subresources...)
	}
	return t, fmt.Errorf("unable to patch %T with %T", t, p.impl)
}

// PatchList is a generic list of JSONPatch operations to apply to a resource
type PatchList struct {
	ops []map[string]any
}

// NewPatchList creates a new empty patch list
func NewPatchList() *PatchList {
	return &PatchList{ops: []map[string]any{}}
}

// Add appends an `add` operation to the patch list
func (pl *PatchList) Add(value any, path ...string) *PatchList {
	if pl == nil {
		pl = NewPatchList()
	}
	if len(path) > 0 {
		for i := range path {
			path[i] = strings.ReplaceAll(path[i], "/", "~1")
		}
		pl.ops = append(pl.ops, map[string]any{
			"op":    "add",
			"value": value,
			"path":  "/" + strings.Join(path, "/"),
		})
	}
	return pl
}

// Remove appends a `remove` operation to the patch list
func (pl *PatchList) Remove(path ...string) *PatchList {
	if pl == nil {
		pl = NewPatchList()
	}
	if len(path) > 0 {
		for i := range path {
			path[i] = strings.ReplaceAll(path[i], "/", "~1")
		}
		pl.ops = append(pl.ops, map[string]any{
			"op":   "remove",
			"path": "/" + strings.Join(path, "/"),
		})
	}
	return pl
}

// ToJSON returns a JSON string containing the patch operations
// This is just a thin wrapper around `json.Marshal`
func (pl *PatchList) ToJSON() (string, error) {
	if pl == nil {
		pl = NewPatchList()
	}
	b, err := json.Marshal(pl.ops)
	return string(b), err
}
