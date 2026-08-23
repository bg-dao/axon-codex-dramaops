//go:build desktop && !production

package main

import "embed"

//go:embed all:frontend/static
var developmentAssets embed.FS

func init() { assets = developmentAssets }
