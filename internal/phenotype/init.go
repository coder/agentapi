// Package phenotype provides initialization for the phenotype-config SDK.
//
// The phenotype-config SDK is a Rust library exposed via CGo. To use it:
//  1. Build the shared library: cargo build --release -p pheno-ffi-go
//  2. Set CGO_LDFLAGS and CGO_CFLAGS to point to the built library and headers
//  3. Call phenotype.Init(repoRoot) at startup
package phenotype

import (
	"os"
	"path/filepath"
)

// Init ensures the .phenotype directory and config.db exist at the given repo root.
// If repoRoot is empty, the current working directory is used.
// This is a lightweight init that creates the directory structure;
// actual SDK calls require the CGo bindings (phenoconfig package).
func Init(repoRoot string) error {
	if repoRoot == "" {
		var err error
		repoRoot, err = os.Getwd()
		if err != nil {
			return err
		}
	}
	dir := filepath.Join(repoRoot, ".phenotype")
	return os.MkdirAll(dir, 0o755)
}
