package main

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"

	"github.com/go-openapi/spec"
	"k8s.io/apiextensions-apiserver/pkg/apiserver"
	"k8s.io/apiserver/pkg/endpoints/openapi"
	"k8s.io/apiserver/pkg/server"
	"k8s.io/kube-aggregator/pkg/apiserver/scheme"
	"k8s.io/kube-openapi/pkg/common"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
	generatedopenapi "k8s.io/kubernetes/pkg/generated/openapi"

	_ "k8s.io/kubernetes/pkg/master" // install APIs
)

var template = `
package openapi

import (
	"bytes"
	"compress/gzip"
	"time"

	jsoniter "github.com/json-iterator/go"
	"k8s.io/klog"
	"k8s.io/kube-openapi/pkg/common"
)

var (
	definitions = map[string]common.OpenAPIDefinition{}
)

func init() {
	start := time.Now()
	defer func() {
		klog.Info("Instantiated OpenAPI definitions in ", time.Now().Sub(start))
	}()

	var json = jsoniter.ConfigCompatibleWithStandardLibrary
	gz, err := gzip.NewReader(bytes.NewBuffer(api))
	if err != nil {
		panic(err)
	}
	if err := json.NewDecoder(gz).Decode(&definitions); err != nil {
		panic(err)
	}
}

func GetOpenAPIDefinitions(_ common.ReferenceCallback) map[string]common.OpenAPIDefinition {
	return definitions
}

`

func main() {
	openAPIConfig := server.DefaultOpenAPIConfig(generatedopenapi.GetOpenAPIDefinitions, openapi.NewDefinitionNamer(legacyscheme.Scheme, apiserver.Scheme, scheme.Scheme))
	definitions := openAPIConfig.GetDefinitions(func(name string) spec.Ref {
		defName, _ := openAPIConfig.GetDefinitionName(name)
		return spec.MustCreateRef("#/definitions/" + common.EscapeJsonPointer(defName))
	})

	buf := &bytes.Buffer{}
	gz := gzip.NewWriter(buf)
	if err := json.NewEncoder(gz).Encode(definitions); err != nil {
		panic(err)
	}
	if err := gz.Close(); err != nil {
		panic(err)
	}
	fmt.Print(template)
	fmt.Print("var api = []byte(\"")
	for _, b := range buf.Bytes() {
		fmt.Printf("\\x%0.2x", b)
	}
	fmt.Print("\")\n")
}
