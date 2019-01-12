package dynamic

import (
	ejson "encoding/json"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var (
	NegotiatedSerializer = negotiatedSerializer{}
)

type negotiatedSerializer struct{}

func (s negotiatedSerializer) SupportedMediaTypes() []runtime.SerializerInfo {
	return []runtime.SerializerInfo{
		{
			MediaType:     "application/json",
			EncodesAsText: true,
			Serializer: dynamicCodec{
				Encoder: unstructured.UnstructuredJSONScheme,
			},
		},
	}
}

func (s negotiatedSerializer) EncoderForVersion(encoder runtime.Encoder, gv runtime.GroupVersioner) runtime.Encoder {
	return encoder
}

func (s negotiatedSerializer) DecoderToVersion(decoder runtime.Decoder, gv runtime.GroupVersioner) runtime.Decoder {
	return decoder
}

type dynamicCodec struct {
	runtime.Encoder
}

func (dynamicCodec) Decode(data []byte, gvk *schema.GroupVersionKind, obj runtime.Object) (runtime.Object, *schema.GroupVersionKind, error) {
	obj, gvk, err := unstructured.UnstructuredJSONScheme.Decode(data, gvk, obj)
	if err != nil {
		return nil, nil, err
	}

	if _, ok := obj.(*metav1.Status); !ok && strings.ToLower(gvk.Kind) == "status" {
		obj = &metav1.Status{}
		err := ejson.Unmarshal(data, obj)
		if err != nil {
			return nil, nil, err
		}
	}

	return obj, gvk, nil
}
