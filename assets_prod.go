//go:build desktop && production

package main

import "embed"

//go:embed all:frontend/dist
var productionAssets embed.FS

func init() { assets = productionAssets }
