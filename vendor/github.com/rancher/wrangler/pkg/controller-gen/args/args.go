package args

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/gengo/types"
)

type CustomArgs struct {
	Package      string
	TypesByGroup map[schema.GroupVersion][]*types.Name
	Options      Options
	OutputBase   string
}

type Options struct {
	OutputPackage string
	Groups        map[string]Group
	Boilerplate   string
	GenMocks      bool
}

type Group struct {
	Types            []interface{}
	GenerateTypes    bool
	PackageName      string
	ClientSetPackage string
	ListersPackage   string
	InformersPackage string
}
