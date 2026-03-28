package main

import (
	"github.com/coder/paralleltestctx/pkg/paralleltestctx"
	"golang.org/x/tools/go/analysis/singlechecker"
)

func main() { singlechecker.Main(paralleltestctx.Analyzer()) }
