package util

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// patcher is an interface for patching a given type, without context or options
type patcher[T any] interface {
	Patch(name string, pt types.PatchType, data []byte, subresources ...string) (T, error)
}

// contextPatcher is an interface for patching a given type, with context and options
type contextPatcher[T any] interface {
	Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions, subresources ...string) (T, error)
}

// PatchList is a list of JSONPatch operations to apply to a resource of a particular type
type PatchList[T any] struct {
	ops []map[string]any
}

// NewPatchList creates a new typed PatchList
func NewPatchList[T any]() *PatchList[T] {
	return &PatchList[T]{}
}

// Add appends an `add` operation to the patch list
func (pl *PatchList[T]) Add(value any, path ...string) *PatchList[T] {
	if pl != nil && len(path) > 0 {
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
func (pl *PatchList[T]) Remove(path ...string) *PatchList[T] {
	if pl != nil && len(path) > 0 {
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
func (pl *PatchList[T]) ToJSON() (string, error) {
	if pl == nil || len(pl.ops) == 0 {
		return "", nil
	}
	b, err := json.Marshal(pl.ops)
	return string(b), err
}

// ApplyTo applies the patch operations to the given resource, using the provided patcher `p`
func (pl *PatchList[T]) ApplyTo(ctx context.Context, p any, name string, subresources ...string) error {
	if pl == nil || len(pl.ops) == 0 {
		return nil
	}
	b, err := json.Marshal(pl.ops)
	if err != nil {
		return err
	}
	if patch, ok := p.(contextPatcher[T]); ok {
		_, err := patch.Patch(ctx, name, types.JSONPatchType, b, metav1.PatchOptions{}, subresources...)
		return err
	}
	if patch, ok := p.(patcher[T]); ok {
		_, err := patch.Patch(name, types.JSONPatchType, b, subresources...)
		return err
	}
	return fmt.Errorf("unable to patch %v with %T", T, p)
}
