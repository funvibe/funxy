package main

import (
	"path/filepath"
	"strings"

	"github.com/funvibe/funxy/internal/modules"
)

type lspModuleLoader struct {
	root string
	base *modules.Loader
}

func newLspModuleLoader(root string) *lspModuleLoader {
	return &lspModuleLoader{
		root: root,
		base: modules.NewLoader(),
	}
}

func (l *lspModuleLoader) GetModule(path string) (interface{}, error) {
	resolved := l.resolvePath(path)
	return l.base.GetModule(resolved)
}

func (l *lspModuleLoader) GetModuleByPackageName(name string) interface{} {
	return l.base.GetModuleByPackageName(name)
}

func (l *lspModuleLoader) resolvePath(path string) string {
	if path == "" {
		return path
	}
	if path == "lib" || strings.HasPrefix(path, "lib/") {
		return path
	}
	if filepath.IsAbs(path) {
		return path
	}
	if l.root != "" {
		return filepath.Join(l.root, path)
	}
	return path
}
