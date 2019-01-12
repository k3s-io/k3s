/*
   Copyright The containerd Authors.

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

package command

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"syscall"
	"text/template"

	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/urfave/cli"
)

var ociHook = cli.Command{
	Name:  "oci-hook",
	Usage: "provides a base for OCI runtime hooks to allow arguments to be injected.",
	Action: func(context *cli.Context) error {
		state, err := loadHookState(os.Stdin)
		if err != nil {
			return err
		}
		spec, err := loadSpec(state.Bundle)
		if err != nil {
			return err
		}
		var (
			ctx  = newTemplateContext(state, spec)
			args = []string(context.Args())
			env  = os.Environ()
		)
		if err := newList(&args).render(ctx); err != nil {
			return err
		}
		if err := newList(&env).render(ctx); err != nil {
			return err
		}
		return syscall.Exec(args[0], args, env)
	},
}

type hookSpec struct {
	Root struct {
		Path string `json:"path"`
	} `json:"root"`
}

func loadSpec(bundle string) (*hookSpec, error) {
	f, err := os.Open(filepath.Join(bundle, "config.json"))
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var s hookSpec
	if err := json.NewDecoder(f).Decode(&s); err != nil {
		return nil, err
	}
	return &s, nil
}

func loadHookState(r io.Reader) (*specs.State, error) {
	var s specs.State
	if err := json.NewDecoder(r).Decode(&s); err != nil {
		return nil, err
	}
	return &s, nil
}

func newTemplateContext(state *specs.State, spec *hookSpec) *templateContext {
	t := &templateContext{
		state: state,
		root:  spec.Root.Path,
	}
	t.funcs = template.FuncMap{
		"id":         t.id,
		"bundle":     t.bundle,
		"rootfs":     t.rootfs,
		"pid":        t.pid,
		"annotation": t.annotation,
		"status":     t.status,
	}
	return t
}

type templateContext struct {
	state *specs.State
	root  string
	funcs template.FuncMap
}

func (t *templateContext) id() string {
	return t.state.ID
}

func (t *templateContext) bundle() string {
	return t.state.Bundle
}

func (t *templateContext) rootfs() string {
	if filepath.IsAbs(t.root) {
		return t.root
	}
	return filepath.Join(t.state.Bundle, t.root)
}

func (t *templateContext) pid() int {
	return t.state.Pid
}

func (t *templateContext) annotation(k string) string {
	return t.state.Annotations[k]
}

func (t *templateContext) status() string {
	return t.state.Status
}

func render(ctx *templateContext, source string, out io.Writer) error {
	t, err := template.New("oci-hook").Funcs(ctx.funcs).Parse(source)
	if err != nil {
		return err
	}
	return t.Execute(out, ctx)
}

func newList(l *[]string) *templateList {
	return &templateList{
		l: l,
	}
}

type templateList struct {
	l *[]string
}

func (l *templateList) render(ctx *templateContext) error {
	buf := bytes.NewBuffer(nil)
	for i, s := range *l.l {
		buf.Reset()
		if err := render(ctx, s, buf); err != nil {
			return err
		}
		(*l.l)[i] = buf.String()
	}
	buf.Reset()
	return nil
}
