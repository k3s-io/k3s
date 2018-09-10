/*
Copyright 2018 The Kubernetes Authors.

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

package tableconvertor

import (
	"bytes"

	"k8s.io/apimachinery/pkg/api/meta"
	metatable "k8s.io/apimachinery/pkg/api/meta/table"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	metav1beta1 "k8s.io/apimachinery/pkg/apis/meta/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
	genericapirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"
	"k8s.io/client-go/util/jsonpath"
)

var swaggerMetadataDescriptions = metav1.ObjectMeta{}.SwaggerDoc()

// New creates a new table convertor for the provided OpenAPI schema. If the printer definition cannot be parsed,
// error will be returned along with a default table convertor.
func New(obj interface{}) (rest.TableConvertor, error) {
	headers := []metav1beta1.TableColumnDefinition{
		{Name: "Name", Type: "string", Format: "name", Description: swaggerMetadataDescriptions["name"]},
		{Name: "Created At", Type: "date", Description: swaggerMetadataDescriptions["creationTimestamp"]},
	}
	c := &convertor{
		headers: headers,
	}
	return c, nil
}

type convertor struct {
	headers []metav1beta1.TableColumnDefinition
	columns []*jsonpath.JSONPath
}

func (c *convertor) ConvertToTable(ctx genericapirequest.Context, obj runtime.Object, tableOptions runtime.Object) (*metav1beta1.Table, error) {
	table := &metav1beta1.Table{
		ColumnDefinitions: c.headers,
	}
	if m, err := meta.ListAccessor(obj); err == nil {
		table.ResourceVersion = m.GetResourceVersion()
		table.SelfLink = m.GetSelfLink()
		table.Continue = m.GetContinue()
	} else {
		if m, err := meta.CommonAccessor(obj); err == nil {
			table.ResourceVersion = m.GetResourceVersion()
			table.SelfLink = m.GetSelfLink()
		}
	}

	var err error
	buf := &bytes.Buffer{}
	table.Rows, err = metatable.MetaToTableRow(obj, func(obj runtime.Object, m metav1.Object, name, age string) ([]interface{}, error) {
		cells := make([]interface{}, 2, 2+len(c.columns))
		cells[0] = name
		cells[1] = age
		for _, column := range c.columns {
			if err := column.Execute(buf, obj); err != nil {
				cells = append(cells, nil)
				continue
			}
			cells = append(cells, buf.String())
			buf.Reset()
		}
		return cells, nil
	})
	return table, err
}
