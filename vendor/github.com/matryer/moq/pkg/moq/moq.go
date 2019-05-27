package moq

import (
	"bytes"
	"errors"
	"fmt"
	"go/format"
	"go/types"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"text/template"

	"golang.org/x/tools/go/packages"
)

// This list comes from the golint codebase. Golint will complain about any of
// these being mixed-case, like "Id" instead of "ID".
var golintInitialisms = []string{
	"ACL",
	"API",
	"ASCII",
	"CPU",
	"CSS",
	"DNS",
	"EOF",
	"GUID",
	"HTML",
	"HTTP",
	"HTTPS",
	"ID",
	"IP",
	"JSON",
	"LHS",
	"QPS",
	"RAM",
	"RHS",
	"RPC",
	"SLA",
	"SMTP",
	"SQL",
	"SSH",
	"TCP",
	"TLS",
	"TTL",
	"UDP",
	"UI",
	"UID",
	"UUID",
	"URI",
	"URL",
	"UTF8",
	"VM",
	"XML",
	"XMPP",
	"XSRF",
	"XSS",
}

// Mocker can generate mock structs.
type Mocker struct {
	srcPkg  *packages.Package
	tmpl    *template.Template
	pkgName string
	pkgPath string
	// importByPath of the format key:path value:alias
	importByPath map[string]string
	// importByAlias of the format key:alias value:path
	importByAlias map[string]string
}

// New makes a new Mocker for the specified package directory.
func New(src, packageName string) (*Mocker, error) {
	srcPkg, err := pkgInfoFromPath(src, packages.LoadSyntax)
	if err != nil {
		return nil, fmt.Errorf("Couldn't load source package: %s", err)
	}
	pkgPath := srcPkg.PkgPath

	if len(packageName) == 0 {
		packageName = srcPkg.Name
	} else {
		mockPkgPath := filepath.Join(src, packageName)
		if _, err := os.Stat(mockPkgPath); os.IsNotExist(err) {
			os.Mkdir(mockPkgPath, os.ModePerm)
		}
		mockPkg, err := pkgInfoFromPath(mockPkgPath, packages.LoadFiles)
		if err != nil {
			return nil, fmt.Errorf("Couldn't load mock package: %s", err)
		}
		pkgPath = mockPkg.PkgPath
	}

	tmpl, err := template.New("moq").Funcs(templateFuncs).Parse(moqTemplate)
	if err != nil {
		return nil, err
	}
	return &Mocker{
		tmpl:          tmpl,
		srcPkg:        srcPkg,
		pkgName:       packageName,
		pkgPath:       pkgPath,
		importByPath:  make(map[string]string),
		importByAlias: make(map[string]string),
	}, nil
}

// Mock generates a mock for the specified interface name.
func (m *Mocker) Mock(w io.Writer, name ...string) error {
	if len(name) == 0 {
		return errors.New("must specify one interface")
	}

	doc := doc{
		PackageName: m.pkgName,
		Imports:     moqImports,
	}

	// Add sync first to ensure it doesn't get an alias which will break the template
	m.addSyncImport()

	var syncNeeded bool

	tpkg := m.srcPkg.Types
	for _, n := range name {
		iface := tpkg.Scope().Lookup(n)
		if iface == nil {
			return fmt.Errorf("cannot find interface %s", n)
		}
		if !types.IsInterface(iface.Type()) {
			return fmt.Errorf("%s (%s) not an interface", n, iface.Type().String())
		}
		iiface := iface.Type().Underlying().(*types.Interface).Complete()
		obj := obj{
			InterfaceName: n,
		}
		for i := 0; i < iiface.NumMethods(); i++ {
			syncNeeded = true
			meth := iiface.Method(i)
			sig := meth.Type().(*types.Signature)
			method := &method{
				Name: meth.Name(),
			}
			obj.Methods = append(obj.Methods, method)
			method.Params = m.extractArgs(sig, sig.Params(), "in%d")
			method.Returns = m.extractArgs(sig, sig.Results(), "out%d")
		}
		doc.Objects = append(doc.Objects, obj)
	}

	if !syncNeeded {
		delete(m.importByAlias, "sync")
		delete(m.importByPath, "sync")
	}

	if tpkg.Name() != m.pkgName {
		if _, ok := m.importByPath[tpkg.Path()]; !ok {
			alias := m.getUniqueAlias(tpkg.Name())
			m.importByAlias[alias] = tpkg.Path()
			m.importByPath[tpkg.Path()] = alias
		}
		doc.SourcePackagePrefix = m.importByPath[tpkg.Path()] + "."
	}

	for alias, path := range m.importByAlias {
		aliasImport := fmt.Sprintf(`%s "%s"`, alias, stripVendorPath(path))
		doc.Imports = append(doc.Imports, aliasImport)
	}

	var buf bytes.Buffer
	err := m.tmpl.Execute(&buf, doc)
	if err != nil {
		return err
	}
	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		return fmt.Errorf("go/format: %s", err)
	}
	if _, err := w.Write(formatted); err != nil {
		return err
	}
	return nil
}

func (m *Mocker) addSyncImport() {
	if _, ok := m.importByPath["sync"]; !ok {
		m.importByAlias["sync"] = "sync"
		m.importByPath["sync"] = "sync"
	}
}

func (m *Mocker) packageQualifier(pkg *types.Package) string {
	if m.pkgPath == pkg.Path() {
		return ""
	}
	path := pkg.Path()
	if pkg.Path() == "." {
		wd, err := os.Getwd()
		if err == nil {
			path = stripGopath(wd)
		}
	}

	if alias, ok := m.importByPath[path]; ok {
		return alias
	}

	alias := pkg.Name()

	if _, ok := m.importByAlias[alias]; ok {
		alias = m.getUniqueAlias(alias)
	}

	m.importByAlias[alias] = path
	m.importByPath[path] = alias

	return alias
}

func (m *Mocker) getUniqueAlias(alias string) string {
	for i := 0; ; i++ {
		newAlias := alias + string('a'+byte(i))
		if _, exists := m.importByAlias[newAlias]; !exists {
			return newAlias
		}
	}
}

func (m *Mocker) extractArgs(sig *types.Signature, list *types.Tuple, nameFormat string) []*param {
	var params []*param
	listLen := list.Len()
	for ii := 0; ii < listLen; ii++ {
		p := list.At(ii)
		name := p.Name()
		if name == "" {
			name = fmt.Sprintf(nameFormat, ii+1)
		}
		typename := types.TypeString(p.Type(), m.packageQualifier)
		// check for final variadic argument
		variadic := sig.Variadic() && ii == listLen-1 && typename[0:2] == "[]"
		param := &param{
			Name:     name,
			Type:     typename,
			Variadic: variadic,
		}
		params = append(params, param)
	}
	return params
}

func pkgInfoFromPath(src string, mode packages.LoadMode) (*packages.Package, error) {
	conf := packages.Config{
		Mode: mode,
		Dir:  src,
	}
	pkgs, err := packages.Load(&conf)
	if err != nil {
		return nil, err
	}
	if len(pkgs) == 0 {
		return nil, errors.New("No packages found")
	}
	if len(pkgs) > 1 {
		return nil, errors.New("More than one package was found")
	}
	return pkgs[0], nil
}

type doc struct {
	PackageName         string
	SourcePackagePrefix string
	Objects             []obj
	Imports             []string
}

type obj struct {
	InterfaceName string
	Methods       []*method
}
type method struct {
	Name    string
	Params  []*param
	Returns []*param
}

func (m *method) Arglist() string {
	params := make([]string, len(m.Params))
	for i, p := range m.Params {
		params[i] = p.String()
	}
	return strings.Join(params, ", ")
}

func (m *method) ArgCallList() string {
	params := make([]string, len(m.Params))
	for i, p := range m.Params {
		params[i] = p.CallName()
	}
	return strings.Join(params, ", ")
}

func (m *method) ReturnArglist() string {
	params := make([]string, len(m.Returns))
	for i, p := range m.Returns {
		params[i] = p.TypeString()
	}
	if len(m.Returns) > 1 {
		return fmt.Sprintf("(%s)", strings.Join(params, ", "))
	}
	return strings.Join(params, ", ")
}

type param struct {
	Name     string
	Type     string
	Variadic bool
}

func (p param) String() string {
	return fmt.Sprintf("%s %s", p.Name, p.TypeString())
}

func (p param) CallName() string {
	if p.Variadic {
		return p.Name + "..."
	}
	return p.Name
}

func (p param) TypeString() string {
	if p.Variadic {
		return "..." + p.Type[2:]
	}
	return p.Type
}

var templateFuncs = template.FuncMap{
	"Exported": func(s string) string {
		if s == "" {
			return ""
		}
		for _, initialism := range golintInitialisms {
			if strings.ToUpper(s) == initialism {
				return initialism
			}
		}
		return strings.ToUpper(s[0:1]) + s[1:]
	},
}

// stripVendorPath strips the vendor dir prefix from a package path.
// For example we might encounter an absolute path like
// github.com/foo/bar/vendor/github.com/pkg/errors which is resolved
// to github.com/pkg/errors.
func stripVendorPath(p string) string {
	parts := strings.Split(p, "/vendor/")
	if len(parts) == 1 {
		return p
	}
	return strings.TrimLeft(path.Join(parts[1:]...), "/")
}

// stripGopath takes the directory to a package and remove the gopath to get the
// canonical package name.
//
// taken from https://github.com/ernesto-jimenez/gogen
// Copyright (c) 2015 Ernesto Jiménez
func stripGopath(p string) string {
	for _, gopath := range gopaths() {
		p = strings.TrimPrefix(p, path.Join(gopath, "src")+"/")
	}
	return p
}

func gopaths() []string {
	return strings.Split(os.Getenv("GOPATH"), string(filepath.ListSeparator))
}
