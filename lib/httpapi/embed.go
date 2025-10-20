package httpapi

import (
	"embed"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"strings"

	"github.com/spf13/afero"
	"golang.org/x/xerrors"
)

//go:embed chat/*
var chatStaticFiles embed.FS

// This must be kept in sync with the BASE_PATH in the Makefile.
const magicBasePath = "/magic-base-path-placeholder"

func createModifiedFS(baseFS fs.FS, oldBasePath string, newBasePath string) (*afero.HttpFs, error) {
	ro := afero.FromIOFS{FS: baseFS}
	overlay := afero.NewMemMapFs()
	newFS := afero.NewCopyOnWriteFs(ro, overlay)

	if err := afero.Walk(ro, ".", func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return xerrors.Errorf("failed to walk: %w", err)
		}
		if info.IsDir() {
			return nil
		}
		byteContents, err := afero.ReadFile(ro, path)
		if err != nil {
			return xerrors.Errorf("failed to read file: %w", err)
		}
		contents := string(byteContents)
		if newBasePath == "/" {
			contents = strings.ReplaceAll(contents, oldBasePath+"/", newBasePath)
		}
		contents = strings.ReplaceAll(contents, oldBasePath, newBasePath)
		if err := afero.WriteFile(overlay, path, []byte(contents), 0644); err != nil {
			return xerrors.Errorf("failed to write file: %w", err)
		}
		return nil
	}); err != nil {
		return nil, xerrors.Errorf("afero.Walk: %w", err)
	}

	return afero.NewHttpFs(newFS), nil
}

// FileServerWithIndexFallback creates a file server that serves the given filesystem
// and falls back to index.html for any path that doesn't match a file
func FileServerWithIndexFallback(chatBasePath string) http.Handler {
	subFS, err := fs.Sub(chatStaticFiles, "chat")
	if err != nil {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, fmt.Sprintf("failed to get subfs: %s", err), http.StatusInternalServerError)
		})
	}
	chatFS, err := createModifiedFS(subFS, magicBasePath, chatBasePath)
	if err != nil {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, fmt.Sprintf("failed to create modified fs: %s", err), http.StatusInternalServerError)
		})
	}
	fileServer := http.FileServer(chatFS.Dir("."))
	isChatDirEmpty := false
	if _, err := chatFS.Open("index.html"); err != nil {
		isChatDirEmpty = true
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isChatDirEmpty {
			http.Error(w,
				"Looks like you're running an agentapi build without the chat UI. To rebuild the binary with the UI files embedded, run `make build`.",
				http.StatusNotFound)
			return
		}
		path := r.URL.Path
		trimmedPath := strings.TrimPrefix(path, "/")
		if trimmedPath == "" {
			trimmedPath = "index.html"
		}

		// Try to serve the file directly
		f, err := chatFS.Open(trimmedPath)
		if err == nil {
			defer func() {
				_ = f.Close()
			}()
			fileServer.ServeHTTP(w, r)
			return
		}

		// If file doesn't exist, serve 404.html for any path
		if os.IsNotExist(err) {
			r2 := new(http.Request)
			*r2 = *r
			r2.URL.Path = "/404.html"
			fileServer.ServeHTTP(w, r2)
			return
		}

		// For other errors, return the error as is
		http.Error(w, fmt.Sprintf("failed to serve file: %s", err), http.StatusInternalServerError)
	})
}
