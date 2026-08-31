package main

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/actions-precompiled/foundation"
)

func fixSiblingInstallNames(ctx context.Context, deps foundation.Deps, res string) error {
	lib := filepath.Join(res, "lib")
	bundled, err := bundledLibNames(deps, lib)
	if err != nil {
		return err
	}
	files, err := deps.FS.Glob(filepath.Join(lib, "*"))
	if err != nil {
		return err
	}
	for _, f := range files {
		base := filepath.Base(f)
		if !strings.HasSuffix(base, ".dylib") && !strings.HasSuffix(base, ".so") {
			continue
		}
		if err := rewriteInstallNames(ctx, deps, f, "@loader_path/", bundled); err != nil {
			return err
		}
	}
	return nil
}

func bundledLibNames(deps foundation.Deps, lib string) (map[string]bool, error) {
	files, err := deps.FS.Glob(filepath.Join(lib, "*"))
	if err != nil {
		return nil, err
	}
	out := make(map[string]bool)
	for _, f := range files {
		base := filepath.Base(f)
		if strings.HasSuffix(base, ".dylib") || strings.HasSuffix(base, ".so") {
			out[base] = true
		}
	}
	return out, nil
}

func rewriteInstallNames(ctx context.Context, deps foundation.Deps, file, prefix string, bundled map[string]bool) error {
	out, err := deps.Runner.Output(ctx, "otool", "-L", file)
	if err != nil {
		return fmt.Errorf("otool -L %s: %w", filepath.Base(file), err)
	}
	id, depsList := parseOtoolL(out)
	if id != "" {
		name := filepath.Base(id)
		want := prefix + name
		if id != want && (bundled[name] || isForeignInstallName(id)) {
			deps.Logf("install_name_tool -id %s %s", want, filepath.Base(file))
			if err := deps.Runner.Run(ctx, "install_name_tool", "-id", want, file); err != nil {
				return fmt.Errorf("install_name_tool -id %s: %w", filepath.Base(file), err)
			}
		}
	}
	for _, dep := range depsList {
		name := filepath.Base(dep)
		if !bundled[name] {
			continue
		}
		want := prefix + name
		if dep == want {
			continue
		}
		deps.Logf("install_name_tool -change %s -> %s (%s)", dep, want, filepath.Base(file))
		if err := deps.Runner.Run(ctx, "install_name_tool", "-change", dep, want, file); err != nil {
			return fmt.Errorf("install_name_tool -change %s: %w", filepath.Base(file), err)
		}
	}
	return nil
}

func parseOtoolL(out string) (id string, deps []string) {
	lines := strings.Split(out, "\n")
	sawHeader := false
	for _, line := range lines {
		s := strings.TrimSpace(line)
		if s == "" {
			continue
		}
		if !sawHeader && strings.HasSuffix(s, ":") {
			sawHeader = true
			continue
		}
		if i := strings.Index(s, " ("); i >= 0 {
			s = s[:i]
		}
		if s == "" {
			continue
		}
		if id == "" {
			id = s
			continue
		}
		deps = append(deps, s)
	}
	return id, deps
}

func isForeignInstallName(s string) bool {
	return strings.HasPrefix(s, "@rpath/") ||
		strings.Contains(s, "/opt/homebrew/") ||
		strings.Contains(s, "/usr/local/opt/") ||
		strings.Contains(s, "/usr/local/Cellar/")
}

func libHasBadSiblingPath(otoolOut string) bool {
	_, deps := parseOtoolL(otoolOut)
	for _, dep := range deps {
		if strings.HasPrefix(dep, "/usr/lib/") || strings.HasPrefix(dep, "/System/") {
			continue
		}
		if strings.Contains(dep, "/../") || strings.HasSuffix(dep, "/..") {
			return true
		}
	}
	return false
}
