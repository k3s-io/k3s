package types

import (
	"net/http"

	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types/slice"
)

func (s *Schema) MustCustomizeField(name string, f func(f Field) Field) *Schema {
	field, ok := s.ResourceFields[name]
	if !ok {
		panic("Failed to find field " + name + " on schema " + s.ID)
	}
	s.ResourceFields[name] = f(field)
	return s
}

func (v *APIVersion) Equals(other *APIVersion) bool {
	return v.Version == other.Version &&
		v.Group == other.Group &&
		v.Path == other.Path
}

func (s *Schema) CanList(context *APIContext) error {
	if context == nil {
		if slice.ContainsString(s.CollectionMethods, http.MethodGet) {
			return nil
		}
		return httperror.NewAPIError(httperror.PermissionDenied, "can not list "+s.ID)
	}
	return context.AccessControl.CanList(context, s)
}

func (s *Schema) CanGet(context *APIContext) error {
	if context == nil {
		if slice.ContainsString(s.ResourceMethods, http.MethodGet) {
			return nil
		}
		return httperror.NewAPIError(httperror.PermissionDenied, "can not get "+s.ID)
	}
	return context.AccessControl.CanGet(context, s)
}

func (s *Schema) CanCreate(context *APIContext) error {
	if context == nil {
		if slice.ContainsString(s.CollectionMethods, http.MethodPost) {
			return nil
		}
		return httperror.NewAPIError(httperror.PermissionDenied, "can not create "+s.ID)
	}
	return context.AccessControl.CanCreate(context, s)
}

func (s *Schema) CanUpdate(context *APIContext) error {
	if context == nil {
		if slice.ContainsString(s.ResourceMethods, http.MethodPut) {
			return nil
		}
		return httperror.NewAPIError(httperror.PermissionDenied, "can not update "+s.ID)
	}
	return context.AccessControl.CanUpdate(context, nil, s)
}

func (s *Schema) CanDelete(context *APIContext) error {
	if context == nil {
		if slice.ContainsString(s.ResourceMethods, http.MethodDelete) {
			return nil
		}
		return httperror.NewAPIError(httperror.PermissionDenied, "can not delete "+s.ID)
	}
	return context.AccessControl.CanDelete(context, nil, s)
}
