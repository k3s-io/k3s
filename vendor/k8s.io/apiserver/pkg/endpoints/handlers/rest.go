/*
Copyright 2014 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package handlers

import (
	"context"
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"time"

	grpccodes "google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"

	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	metav1beta1 "k8s.io/apimachinery/pkg/apis/meta/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/apiserver/pkg/authorization/authorizer"
	"k8s.io/apiserver/pkg/endpoints/handlers/fieldmanager"
	"k8s.io/apiserver/pkg/endpoints/handlers/responsewriters"
	"k8s.io/apiserver/pkg/endpoints/metrics"
	"k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/features"
	"k8s.io/apiserver/pkg/registry/rest"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
	"k8s.io/apiserver/pkg/warning"
	"k8s.io/klog/v2"
)

const (
	// 34 chose as a number close to 30 that is likely to be unique enough to jump out at me the next time I see a timeout.
	// Everyone chooses 30.
	requestTimeoutUpperBound = 34 * time.Second
	// DuplicateOwnerReferencesWarningFormat is the warning that a client receives when a create/update request contains
	// duplicate owner reference entries.
	DuplicateOwnerReferencesWarningFormat = ".metadata.ownerReferences contains duplicate entries; API server dedups owner references in 1.20+, and may reject such requests as early as 1.24; please fix your requests; duplicate UID(s) observed: %v"
	// DuplicateOwnerReferencesAfterMutatingAdmissionWarningFormat indicates the duplication was observed
	// after mutating admission.
	// NOTE: For CREATE and UPDATE requests the API server dedups both before and after mutating admission.
	// For PATCH request the API server only dedups after mutating admission.
	DuplicateOwnerReferencesAfterMutatingAdmissionWarningFormat = ".metadata.ownerReferences contains duplicate entries after mutating admission happens; API server dedups owner references in 1.20+, and may reject such requests as early as 1.24; please fix your requests; duplicate UID(s) observed: %v"
)

// RequestScope encapsulates common fields across all RESTful handler methods.
type RequestScope struct {
	Namer ScopeNamer

	Serializer runtime.NegotiatedSerializer
	runtime.ParameterCodec

	// StandardSerializers, if set, restricts which serializers can be used when
	// we aren't transforming the output (into Table or PartialObjectMetadata).
	// Used only by CRDs which do not yet support Protobuf.
	StandardSerializers []runtime.SerializerInfo

	Creater         runtime.ObjectCreater
	Convertor       runtime.ObjectConvertor
	Defaulter       runtime.ObjectDefaulter
	Typer           runtime.ObjectTyper
	UnsafeConvertor runtime.ObjectConvertor
	Authorizer      authorizer.Authorizer

	EquivalentResourceMapper runtime.EquivalentResourceMapper

	TableConvertor rest.TableConvertor
	FieldManager   *fieldmanager.FieldManager

	Resource schema.GroupVersionResource
	Kind     schema.GroupVersionKind

	// AcceptsGroupVersionDelegate is an optional delegate that can be queried about whether a given GVK
	// can be accepted in create or update requests. If nil, only scope.Kind is accepted.
	// Note that this does not enable multi-version support for reads from a single endpoint.
	AcceptsGroupVersionDelegate rest.GroupVersionAcceptor

	Subresource string

	MetaGroupVersion schema.GroupVersion

	// HubGroupVersion indicates what version objects read from etcd or incoming requests should be converted to for in-memory handling.
	HubGroupVersion schema.GroupVersion

	MaxRequestBodyBytes int64
}

func (scope *RequestScope) err(err error, w http.ResponseWriter, req *http.Request) {
	responsewriters.ErrorNegotiated(err, scope.Serializer, scope.Kind.GroupVersion(), w, req)
}

// AcceptsGroupVersion returns true if the specified GroupVersion is allowed
// in create and update requests.
func (scope *RequestScope) AcceptsGroupVersion(gv schema.GroupVersion) bool {
	// If there's a custom acceptor, delegate to it. This is extremely rare.
	if scope.AcceptsGroupVersionDelegate != nil {
		return scope.AcceptsGroupVersionDelegate.AcceptsGroupVersion(gv)
	}
	// Fall back to only allowing the singular Kind. This is the typical behavior.
	return gv == scope.Kind.GroupVersion()
}

func (scope *RequestScope) AllowsMediaTypeTransform(mimeType, mimeSubType string, gvk *schema.GroupVersionKind) bool {
	// some handlers like CRDs can't serve all the mime types that PartialObjectMetadata or Table can - if
	// gvk is nil (no conversion) allow StandardSerializers to further restrict the set of mime types.
	if gvk == nil {
		if len(scope.StandardSerializers) == 0 {
			return true
		}
		for _, info := range scope.StandardSerializers {
			if info.MediaTypeType == mimeType && info.MediaTypeSubType == mimeSubType {
				return true
			}
		}
		return false
	}

	// TODO: this is temporary, replace with an abstraction calculated at endpoint installation time
	if gvk.GroupVersion() == metav1beta1.SchemeGroupVersion || gvk.GroupVersion() == metav1.SchemeGroupVersion {
		switch gvk.Kind {
		case "Table":
			return scope.TableConvertor != nil &&
				mimeType == "application" &&
				(mimeSubType == "json" || mimeSubType == "yaml")
		case "PartialObjectMetadata", "PartialObjectMetadataList":
			// TODO: should delineate between lists and non-list endpoints
			return true
		default:
			return false
		}
	}
	return false
}

func (scope *RequestScope) AllowsServerVersion(version string) bool {
	return version == scope.MetaGroupVersion.Version
}

func (scope *RequestScope) AllowsStreamSchema(s string) bool {
	return s == "watch"
}

var _ admission.ObjectInterfaces = &RequestScope{}

func (r *RequestScope) GetObjectCreater() runtime.ObjectCreater     { return r.Creater }
func (r *RequestScope) GetObjectTyper() runtime.ObjectTyper         { return r.Typer }
func (r *RequestScope) GetObjectDefaulter() runtime.ObjectDefaulter { return r.Defaulter }
func (r *RequestScope) GetObjectConvertor() runtime.ObjectConvertor { return r.Convertor }
func (r *RequestScope) GetEquivalentResourceMapper() runtime.EquivalentResourceMapper {
	return r.EquivalentResourceMapper
}

// ConnectResource returns a function that handles a connect request on a rest.Storage object.
func ConnectResource(connecter rest.Connecter, scope *RequestScope, admit admission.Interface, restPath string, isSubresource bool) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		if isDryRun(req.URL) {
			scope.err(errors.NewBadRequest("dryRun is not supported"), w, req)
			return
		}

		namespace, name, err := scope.Namer.Name(req)
		if err != nil {
			scope.err(err, w, req)
			return
		}
		ctx := req.Context()
		ctx = request.WithNamespace(ctx, namespace)
		ae := request.AuditEventFrom(ctx)
		admit = admission.WithAudit(admit, ae)

		opts, subpath, subpathKey := connecter.NewConnectOptions()
		if err := getRequestOptions(req, scope, opts, subpath, subpathKey, isSubresource); err != nil {
			err = errors.NewBadRequest(err.Error())
			scope.err(err, w, req)
			return
		}
		if admit != nil && admit.Handles(admission.Connect) {
			userInfo, _ := request.UserFrom(ctx)
			// TODO: remove the mutating admission here as soon as we have ported all plugin that handle CONNECT
			if mutatingAdmission, ok := admit.(admission.MutationInterface); ok {
				err = mutatingAdmission.Admit(ctx, admission.NewAttributesRecord(opts, nil, scope.Kind, namespace, name, scope.Resource, scope.Subresource, admission.Connect, nil, false, userInfo), scope)
				if err != nil {
					scope.err(err, w, req)
					return
				}
			}
			if validatingAdmission, ok := admit.(admission.ValidationInterface); ok {
				err = validatingAdmission.Validate(ctx, admission.NewAttributesRecord(opts, nil, scope.Kind, namespace, name, scope.Resource, scope.Subresource, admission.Connect, nil, false, userInfo), scope)
				if err != nil {
					scope.err(err, w, req)
					return
				}
			}
		}
		requestInfo, _ := request.RequestInfoFrom(ctx)
		metrics.RecordLongRunning(req, requestInfo, metrics.APIServerComponent, func() {
			handler, err := connecter.Connect(ctx, name, opts, &responder{scope: scope, req: req, w: w})
			if err != nil {
				scope.err(err, w, req)
				return
			}
			handler.ServeHTTP(w, req)
		})
	}
}

// responder implements rest.Responder for assisting a connector in writing objects or errors.
type responder struct {
	scope *RequestScope
	req   *http.Request
	w     http.ResponseWriter
}

func (r *responder) Object(statusCode int, obj runtime.Object) {
	responsewriters.WriteObjectNegotiated(r.scope.Serializer, r.scope, r.scope.Kind.GroupVersion(), r.w, r.req, statusCode, obj)
}

func (r *responder) Error(err error) {
	r.scope.err(err, r.w, r.req)
}

// transformDecodeError adds additional information into a bad-request api error when a decode fails.
func transformDecodeError(typer runtime.ObjectTyper, baseErr error, into runtime.Object, gvk *schema.GroupVersionKind, body []byte) error {
	objGVKs, _, err := typer.ObjectKinds(into)
	if err != nil {
		return errors.NewBadRequest(err.Error())
	}
	objGVK := objGVKs[0]
	if gvk != nil && len(gvk.Kind) > 0 {
		return errors.NewBadRequest(fmt.Sprintf("%s in version %q cannot be handled as a %s: %v", gvk.Kind, gvk.Version, objGVK.Kind, baseErr))
	}
	summary := summarizeData(body, 30)
	return errors.NewBadRequest(fmt.Sprintf("the object provided is unrecognized (must be of type %s): %v (%s)", objGVK.Kind, baseErr, summary))
}

// setSelfLink sets the self link of an object (or the child items in a list) to the base URL of the request
// plus the path and query generated by the provided linkFunc
func setSelfLink(obj runtime.Object, requestInfo *request.RequestInfo, namer ScopeNamer) error {
	// TODO: SelfLink generation should return a full URL?
	uri, err := namer.GenerateLink(requestInfo, obj)
	if err != nil {
		return nil
	}

	return namer.SetSelfLink(obj, uri)
}

func hasUID(obj runtime.Object) (bool, error) {
	if obj == nil {
		return false, nil
	}
	accessor, err := meta.Accessor(obj)
	if err != nil {
		return false, errors.NewInternalError(err)
	}
	if len(accessor.GetUID()) == 0 {
		return false, nil
	}
	return true, nil
}

// checkName checks the provided name against the request
func checkName(obj runtime.Object, name, namespace string, namer ScopeNamer) error {
	objNamespace, objName, err := namer.ObjectName(obj)
	if err != nil {
		return errors.NewBadRequest(fmt.Sprintf(
			"the name of the object (%s based on URL) was undeterminable: %v", name, err))
	}
	if objName != name {
		return errors.NewBadRequest(fmt.Sprintf(
			"the name of the object (%s) does not match the name on the URL (%s)", objName, name))
	}
	if len(namespace) > 0 {
		if len(objNamespace) > 0 && objNamespace != namespace {
			return errors.NewBadRequest(fmt.Sprintf(
				"the namespace of the object (%s) does not match the namespace on the request (%s)", objNamespace, namespace))
		}
	}

	return nil
}

// dedupOwnerReferences dedups owner references over the entire entry.
// NOTE: We don't know enough about the existing cases of owner references
// sharing the same UID but different fields. Nor do we know what might break.
// In the future we may just dedup/reject owner references with the same UID.
func dedupOwnerReferences(refs []metav1.OwnerReference) ([]metav1.OwnerReference, []string) {
	var result []metav1.OwnerReference
	var duplicates []string
	seen := make(map[types.UID]struct{})
	for _, ref := range refs {
		_, ok := seen[ref.UID]
		// Short-circuit if we haven't seen the UID before. Otherwise
		// check the entire list we have so far.
		if !ok || !hasOwnerReference(result, ref) {
			seen[ref.UID] = struct{}{}
			result = append(result, ref)
		} else {
			duplicates = append(duplicates, string(ref.UID))
		}
	}
	return result, duplicates
}

// hasOwnerReference returns true if refs has an item equal to ref. The function
// focuses on semantic equality instead of memory equality, to catch duplicates
// with different pointer addresses. The function uses apiequality.Semantic
// instead of implementing its own comparison, to tolerate API changes to
// metav1.OwnerReference.
// NOTE: This is expensive, but we accept it because we've made sure it only
// happens to owner references containing duplicate UIDs, plus typically the
// number of items in the list should be small.
func hasOwnerReference(refs []metav1.OwnerReference, ref metav1.OwnerReference) bool {
	for _, r := range refs {
		if apiequality.Semantic.DeepEqual(r, ref) {
			return true
		}
	}
	return false
}

// dedupOwnerReferencesAndAddWarning dedups owner references in the object metadata.
// If duplicates are found, the function records a warning to the provided context.
func dedupOwnerReferencesAndAddWarning(obj runtime.Object, requestContext context.Context, afterMutatingAdmission bool) {
	accessor, err := meta.Accessor(obj)
	if err != nil {
		// The object doesn't have metadata. Nothing we need to do here.
		return
	}
	refs := accessor.GetOwnerReferences()
	deduped, duplicates := dedupOwnerReferences(refs)
	if len(duplicates) > 0 {
		// NOTE: For CREATE and UPDATE requests the API server dedups both before and after mutating admission.
		// For PATCH request the API server only dedups after mutating admission.
		format := DuplicateOwnerReferencesWarningFormat
		if afterMutatingAdmission {
			format = DuplicateOwnerReferencesAfterMutatingAdmissionWarningFormat
		}
		warning.AddWarning(requestContext, "", fmt.Sprintf(format,
			strings.Join(duplicates, ", ")))
		accessor.SetOwnerReferences(deduped)
	}
}

// setObjectSelfLink sets the self link of an object as needed.
// TODO: remove the need for the namer LinkSetters by requiring objects implement either Object or List
//   interfaces
func setObjectSelfLink(ctx context.Context, obj runtime.Object, req *http.Request, namer ScopeNamer) error {
	if utilfeature.DefaultFeatureGate.Enabled(features.RemoveSelfLink) {
		// Ensure that for empty lists we don't return <nil> items.
		if meta.IsListType(obj) && meta.LenList(obj) == 0 {
			if err := meta.SetList(obj, []runtime.Object{}); err != nil {
				return err
			}
		}
		return nil
	}

	// We only generate list links on objects that implement ListInterface - historically we duck typed this
	// check via reflection, but as we move away from reflection we require that you not only carry Items but
	// ListMeta into order to be identified as a list.
	if !meta.IsListType(obj) {
		requestInfo, ok := request.RequestInfoFrom(ctx)
		if !ok {
			return fmt.Errorf("missing requestInfo")
		}
		return setSelfLink(obj, requestInfo, namer)
	}

	uri, err := namer.GenerateListLink(req)
	if err != nil {
		return err
	}
	if err := namer.SetSelfLink(obj, uri); err != nil {
		klog.V(4).InfoS("Unable to set self link on object", "error", err)
	}
	requestInfo, ok := request.RequestInfoFrom(ctx)
	if !ok {
		return fmt.Errorf("missing requestInfo")
	}

	count := 0
	err = meta.EachListItem(obj, func(obj runtime.Object) error {
		count++
		return setSelfLink(obj, requestInfo, namer)
	})

	if count == 0 {
		if err := meta.SetList(obj, []runtime.Object{}); err != nil {
			return err
		}
	}

	return err
}

func summarizeData(data []byte, maxLength int) string {
	switch {
	case len(data) == 0:
		return "<empty>"
	case data[0] == '{':
		if len(data) > maxLength {
			return string(data[:maxLength]) + " ..."
		}
		return string(data)
	default:
		if len(data) > maxLength {
			return hex.EncodeToString(data[:maxLength]) + " ..."
		}
		return hex.EncodeToString(data)
	}
}

func limitedReadBody(req *http.Request, limit int64) ([]byte, error) {
	defer req.Body.Close()
	if limit <= 0 {
		return ioutil.ReadAll(req.Body)
	}
	lr := &io.LimitedReader{
		R: req.Body,
		N: limit + 1,
	}
	data, err := ioutil.ReadAll(lr)
	if err != nil {
		return nil, err
	}
	if lr.N <= 0 {
		return nil, errors.NewRequestEntityTooLargeError(fmt.Sprintf("limit is %d", limit))
	}
	return data, nil
}

func isDryRun(url *url.URL) bool {
	return len(url.Query()["dryRun"]) != 0
}

type etcdError interface {
	Code() grpccodes.Code
	Error() string
}

type grpcError interface {
	GRPCStatus() *grpcstatus.Status
}

func isTooLargeError(err error) bool {
	if err != nil {
		if etcdErr, ok := err.(etcdError); ok {
			if etcdErr.Code() == grpccodes.InvalidArgument && etcdErr.Error() == "etcdserver: request is too large" {
				return true
			}
		}
		if grpcErr, ok := err.(grpcError); ok {
			if grpcErr.GRPCStatus().Code() == grpccodes.ResourceExhausted && strings.Contains(grpcErr.GRPCStatus().Message(), "trying to send message larger than max") {
				return true
			}
		}
	}
	return false
}
