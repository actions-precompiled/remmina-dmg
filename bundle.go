package main

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/actions-precompiled/foundation"
)

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

	if err := compileLauncher(ctx, deps, filepath.Join(macos, "Remmina")); err != nil {
		return err
	}
	plist := fmt.Sprintf(infoPlist, version, version)
	if err := deps.FS.WriteFile(filepath.Join(appDir, "Contents", "Info.plist"), []byte(plist), 0o644); err != nil {
		return err
	}
	if err := installAppIcon(ctx, deps, res); err != nil {
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

	return signApp(ctx, deps, appDir)
}

const remminaAppPNG = "org.remmina.Remmina.png"

// iconsetSlots maps installed hicolor sizes to iconutil filenames.
func iconsetSlots() []struct{ size, name string } {
	return []struct{ size, name string }{
		{"16", "icon_16x16.png"},
		{"32", "icon_16x16@2x.png"},
		{"32", "icon_32x32.png"},
		{"64", "icon_32x32@2x.png"},
		{"128", "icon_128x128.png"},
		{"256", "icon_128x128@2x.png"},
		{"256", "icon_256x256.png"},
		{"512", "icon_256x256@2x.png"},
		{"512", "icon_512x512.png"},
	}
}

func installAppIcon(ctx context.Context, deps foundation.Deps, res string) error {
	hicolor := filepath.Join(res, "share", "icons", "hicolor")
	iconset := filepath.Join(res, "AppIcon.iconset")
	deps.RemoveAllLog(iconset, "iconset")
	if err := deps.FS.MkdirAll(iconset, 0o755); err != nil {
		return err
	}
	for _, slot := range iconsetSlots() {
		src := filepath.Join(hicolor, slot.size+"x"+slot.size, "apps", remminaAppPNG)
		if _, err := deps.FS.Stat(src); err != nil {
			return fmt.Errorf("%w: %s", ErrAppIconMissing, src)
		}
		if err := deps.Runner.Run(ctx, "cp", src, filepath.Join(iconset, slot.name)); err != nil {
			return fmt.Errorf("icon %s: %w", slot.name, err)
		}
	}
	src512 := filepath.Join(hicolor, "512x512", "apps", remminaAppPNG)
	dst1024 := filepath.Join(iconset, "icon_512x512@2x.png")
	if err := deps.Runner.Run(ctx, "sips", "-z", "1024", "1024", src512, "--out", dst1024); err != nil {
		return fmt.Errorf("sips 1024: %w", err)
	}
	icns := filepath.Join(res, "AppIcon.icns")
	if err := deps.Runner.Run(ctx, "iconutil", "-c", "icns", iconset, "-o", icns); err != nil {
		return fmt.Errorf("iconutil: %w", err)
	}
	deps.RemoveAllLog(iconset, "iconset")
	deps.Logf("app icon: %s", icns)
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
	<key>CFBundleIconFile</key>
	<string>AppIcon</string>
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
