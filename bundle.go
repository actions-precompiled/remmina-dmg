package main

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/actions-precompiled/foundation"
)

const launcherScript = `#!/bin/bash
set -euo pipefail
DIR="$(cd "$(dirname "$0")" && pwd)"
RES="$DIR/../Resources"
export XDG_DATA_DIRS="$RES/share:${XDG_DATA_DIRS:-/usr/share}"
export GSETTINGS_SCHEMA_DIR="$RES/share/glib-2.0/schemas"
export GDK_PIXBUF_MODULEDIR="$RES/lib/gdk-pixbuf-2.0/2.10.0/loaders"
export GTK_DATA_PREFIX="$RES"
export GTK_EXE_PREFIX="$RES"
export GTK_PATH="$RES"
export GI_TYPELIB_PATH="$RES/lib/girepository-1.0${GI_TYPELIB_PATH:+:$GI_TYPELIB_PATH}"
export GIO_MODULE_DIR="$RES/lib/gio/modules"
export PANGO_LIBDIR="$RES/lib"
exec "$RES/bin/remmina" "$@"
`

func assembleApp(ctx context.Context, deps foundation.Deps, prefix, appDir, brew, version string) error {
	macos := filepath.Join(appDir, "Contents", "MacOS")
	res := filepath.Join(appDir, "Contents", "Resources")
	deps.RemoveAllLog(appDir, "remove")
	if err := deps.FS.MkdirAll(macos, 0o755); err != nil {
		return err
	}
	if err := deps.Runner.Run(ctx, "cp", "-a", prefix, res); err != nil {
		return fmt.Errorf("copy prefix into app: %w", err)
	}

	if err := deps.FS.WriteFile(filepath.Join(macos, "Remmina"), []byte(launcherScript), 0o755); err != nil {
		return err
	}
	plist := fmt.Sprintf(infoPlist, version, version)
	if err := deps.FS.WriteFile(filepath.Join(appDir, "Contents", "Info.plist"), []byte(plist), 0o644); err != nil {
		return err
	}

	if err := vendorGTKData(ctx, deps, res, brew); err != nil {
		return err
	}
	if err := bundleDylibs(ctx, deps, res); err != nil {
		return err
	}
	if err := compileSchemas(ctx, deps, res); err != nil {
		return err
	}

	if _, err := deps.Runner.Output(ctx, "which", "codesign"); err == nil {
		deps.Logf("codesign --sign - (ad-hoc, unsigned like rterm)")
		if err := deps.Runner.Run(ctx, "codesign", "--force", "--deep", "--sign", "-", appDir); err != nil {
			return fmt.Errorf("codesign: %w", err)
		}
	}
	return nil
}

func vendorGTKData(ctx context.Context, deps foundation.Deps, res, brew string) error {
	copies := [][2]string{
		{filepath.Join(brew, "share", "glib-2.0", "schemas"), filepath.Join(res, "share", "glib-2.0", "schemas")},
		{filepath.Join(brew, "share", "icons", "Adwaita"), filepath.Join(res, "share", "icons", "Adwaita")},
		{filepath.Join(brew, "share", "icons", "hicolor"), filepath.Join(res, "share", "icons", "hicolor")},
		{filepath.Join(brew, "lib", "gdk-pixbuf-2.0"), filepath.Join(res, "lib", "gdk-pixbuf-2.0")},
		{filepath.Join(brew, "lib", "gio", "modules"), filepath.Join(res, "lib", "gio", "modules")},
		{filepath.Join(brew, "lib", "girepository-1.0"), filepath.Join(res, "lib", "girepository-1.0")},
	}
	for _, pair := range copies {
		if _, err := deps.FS.Stat(pair[0]); err != nil {
			deps.Logf("vendor: skip missing %s", pair[0])
			continue
		}
		if err := deps.FS.MkdirAll(filepath.Dir(pair[1]), 0o755); err != nil {
			return err
		}
		if err := deps.Runner.Run(ctx, "cp", "-a", pair[0], pair[1]); err != nil {
			return fmt.Errorf("copy %s: %w", pair[0], err)
		}
	}
	return nil
}

func bundleDylibs(ctx context.Context, deps foundation.Deps, res string) error {
	lib := filepath.Join(res, "lib")
	if err := deps.FS.MkdirAll(lib, 0o755); err != nil {
		return err
	}
	bins := []string{filepath.Join(res, "bin", "remmina")}
	bins = append(bins, globMachO(deps, filepath.Join(res, "lib", "remmina", "plugins"))...)
	bins = append(bins, globMachO(deps, filepath.Join(res, "lib", "gdk-pixbuf-2.0"))...)
	bins = append(bins, globMachO(deps, filepath.Join(res, "lib", "gio", "modules"))...)
	for _, bin := range bins {
		prefix := loaderPathToLib(res, bin)
		deps.Logf("dylibbundler %s (%s)", bin, prefix)
		args := dylibbundlerArgs(bin, lib, prefix)
		err := deps.Runner.Run(ctx, "dylibbundler", args...)
		if err != nil {
			return fmt.Errorf("%w: %s: %w", ErrDylibbundler, filepath.Base(bin), err)
		}
	}
	return nil
}

func dylibbundlerArgs(bin, dest, prefix string) []string {
	// -of overwrites dest files. Do not pass -od: that wipes dest (and
	// remmina plugins living under lib/remmina/plugins).
	return []string{"-of", "-cd", "-b", "-ns", "-x", bin, "-d", dest, "-p", prefix}
}

func loaderPathToLib(res, bin string) string {
	rel, err := filepath.Rel(filepath.Dir(bin), filepath.Join(res, "lib"))
	if err != nil {
		return "@loader_path/../lib"
	}
	return "@loader_path/" + filepath.ToSlash(rel)
}

func globMachO(deps foundation.Deps, root string) []string {
	if _, err := deps.FS.Stat(root); err != nil {
		return nil
	}
	matches, err := deps.FS.Glob(filepath.Join(root, "*"))
	if err != nil {
		return nil
	}
	var out []string
	for _, p := range matches {
		base := filepath.Base(p)
		if strings.HasSuffix(base, ".so") || strings.HasSuffix(base, ".dylib") {
			out = append(out, p)
		}
	}
	// gdk-pixbuf loaders live one more directory down
	nested, err := deps.FS.Glob(filepath.Join(root, "*", "*", "loaders", "*.so"))
	if err == nil {
		out = append(out, nested...)
	}
	return out
}

func compileSchemas(ctx context.Context, deps foundation.Deps, res string) error {
	dir := filepath.Join(res, "share", "glib-2.0", "schemas")
	if _, err := deps.FS.Stat(dir); err != nil {
		return nil
	}
	if err := deps.Runner.Run(ctx, "glib-compile-schemas", dir); err != nil {
		deps.Logf("glib-compile-schemas: %v (continuing)", err)
	}
	return nil
}

func writeDMG(ctx context.Context, deps foundation.Deps, appDir, dest string) error {
	root := filepath.Join(filepath.Dir(appDir), "dmgroot")
	deps.RemoveAllLog(root, "remove")
	if err := deps.FS.MkdirAll(root, 0o755); err != nil {
		return err
	}
	if err := deps.Runner.Run(ctx, "cp", "-a", appDir, filepath.Join(root, "Remmina.app")); err != nil {
		return fmt.Errorf("stage app into dmgroot: %w", err)
	}
	if err := deps.Runner.Run(ctx, "ln", "-s", "/Applications", filepath.Join(root, "Applications")); err != nil {
		return fmt.Errorf("applications symlink: %w", err)
	}
	if err := deps.FS.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	deps.RemoveAllLog(dest, "remove")
	if err := deps.Runner.Run(ctx, "hdiutil", "create",
		"-volname", "Remmina",
		"-srcfolder", root,
		"-ov",
		"-format", "UDZO",
		dest,
	); err != nil {
		return fmt.Errorf("%w: %w", ErrDMG, err)
	}
	return nil
}

const infoPlist = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>CFBundleDevelopmentRegion</key>
	<string>en</string>
	<key>CFBundleExecutable</key>
	<string>Remmina</string>
	<key>CFBundleIdentifier</key>
	<string>org.remmina.Remmina</string>
	<key>CFBundleInfoDictionaryVersion</key>
	<string>6.0</string>
	<key>CFBundleName</key>
	<string>Remmina</string>
	<key>CFBundlePackageType</key>
	<string>APPL</string>
	<key>CFBundleShortVersionString</key>
	<string>%s</string>
	<key>CFBundleVersion</key>
	<string>%s</string>
	<key>LSMinimumSystemVersion</key>
	<string>12.0</string>
	<key>NSHighResolutionCapable</key>
	<true/>
	<key>NSSupportsAutomaticGraphicsSwitching</key>
	<true/>
</dict>
</plist>
`
