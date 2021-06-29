/*
Copyright 2020 The Kubernetes Authors.

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

package crictl

import (
	"bytes"
	"encoding/json"
	"strings"
	"text/template"

	"github.com/pkg/errors"
)

func builtinTmplFuncs() template.FuncMap {
	return template.FuncMap{
		"json":  jsonBuiltinTmplFunc,
		"title": strings.Title,
		"lower": strings.ToLower,
		"upper": strings.ToUpper,
	}
}

// jsonBuiltinTmplFunc allows to jsonify result of template execution.
func jsonBuiltinTmplFunc(v interface{}) string {
	o := new(bytes.Buffer)
	enc := json.NewEncoder(o)
	// FIXME(fuweid): should we panic?
	enc.Encode(v)
	return o.String()
}

// tmplExecuteRawJSON executes the template with interface{} with decoded by
// rawJSON string.
func tmplExecuteRawJSON(tmplStr string, rawJSON string) (string, error) {
	dec := json.NewDecoder(
		bytes.NewReader([]byte(rawJSON)),
	)
	dec.UseNumber()

	var raw interface{}
	if err := dec.Decode(&raw); err != nil {
		return "", errors.Wrapf(err, "failed to decode json")
	}

	var o = new(bytes.Buffer)
	tmpl, err := template.New("tmplExecuteRawJSON").Funcs(builtinTmplFuncs()).Parse(tmplStr)
	if err != nil {
		return "", errors.Wrapf(err, "failed to generate go-template")
	}

	// return error if key doesn't exist
	tmpl = tmpl.Option("missingkey=error")
	if err := tmpl.Execute(o, raw); err != nil {
		return "", errors.Wrapf(err, "failed to template data")
	}
	return o.String(), nil
}
