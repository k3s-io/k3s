package data

import (
	"embed"

	"github.com/k3s-io/k3s/pkg/util/bindata"
)

//go:embed embed/*
var embedFS embed.FS

var (
	bd = bindata.Bindata{FS: &embedFS, Prefix: "embed"}

	Asset      = bd.Asset
	AssetNames = bd.AssetNames
)
