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
	if err := checkRemminaIcons(deps, res); err != nil {
		return err
	}
	if err := updateIconCaches(ctx, deps, res); err != nil {
		return err
	}
	if err := writeGTKSettings(deps, res); err != nil {
		return err
	}
	if err := bundleDylibs(ctx, deps, res, brew); err != nil {
		return err
	}
	if err := writePixbufCache(ctx, deps, res, brew); err != nil {
		return err
	}
	return compileSchemas(ctx, deps, res)
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
	destSchemas := filepath.Join(res, "share", "glib-2.0", "schemas")
	copies := [][2]string{
		// Homebrew schemas are Cellar symlinks. Follow them and merge
		// into dest; cp -a of the directory nests or leaves dangling links.
		{filepath.Join(brew, "share", "glib-2.0", "schemas"), destSchemas},
		{filepath.Join(brew, "opt", "gtk+3", "share", "glib-2.0", "schemas"), destSchemas},
		{filepath.Join(brew, "share", "icons", "Adwaita"), filepath.Join(res, "share", "icons", "Adwaita")},
		{filepath.Join(brew, "opt", "adwaita-icon-theme", "share", "icons", "Adwaita"), filepath.Join(res, "share", "icons", "Adwaita")},
		{filepath.Join(brew, "share", "icons", "AdwaitaLegacy"), filepath.Join(res, "share", "icons", "AdwaitaLegacy")},
		{filepath.Join(brew, "opt", "adwaita-icon-theme-legacy", "share", "icons", "AdwaitaLegacy"), filepath.Join(res, "share", "icons", "AdwaitaLegacy")},
		{filepath.Join(brew, "share", "icons", "hicolor"), filepath.Join(res, "share", "icons", "hicolor")},
		{filepath.Join(brew, "lib", "gdk-pixbuf-2.0"), filepath.Join(res, "lib", "gdk-pixbuf-2.0")},
		{filepath.Join(brew, "lib", "gio", "modules"), filepath.Join(res, "lib", "gio", "modules")},
		{filepath.Join(brew, "lib", "girepository-1.0"), filepath.Join(res, "lib", "girepository-1.0")},
	}
	for _, pair := range copies {
		if err := mergeTree(ctx, deps, pair[0], pair[1]); err != nil {
			return err
		}
	}
	return nil
}

func requiredSchemaFiles() []string {
	return []string{
		"org.gtk.Settings.FileChooser.gschema.xml",
	}
}

func mergeTreeArgs(src, dest string) []string {
	// -RL follows Cellar keg symlinks. src/. copies contents into dest
	// instead of nesting dest/<basename> when dest already exists.
	return []string{"-RL", src + "/.", dest}
}

func mergeTree(ctx context.Context, deps foundation.Deps, src, dest string) error {
	if _, err := deps.FS.Stat(src); err != nil {
		deps.Logf("vendor: skip missing %s", src)
		return nil
	}
	if err := deps.FS.MkdirAll(dest, 0o755); err != nil {
		return err
	}
	if err := deps.Runner.Run(ctx, "cp", mergeTreeArgs(src, dest)...); err != nil {
		return fmt.Errorf("copy %s: %w", src, err)
	}
	// Cellar files are often 444; dylibbundler and icon-cache need write.
	_ = deps.Runner.Run(ctx, "chmod", "-R", "u+w", dest)
	return nil
}

func bundleDylibs(ctx context.Context, deps foundation.Deps, res, brew string) error {
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
	if err := vendorMissingRpathLibs(ctx, deps, res, brew); err != nil {
		return err
	}
	if err := fixLoaderInstallNames(ctx, deps, res); err != nil {
		return err
	}
	// dylibbundler reuses -p when rewriting dest libs. Plugin runs use
	// @loader_path/../.. and poison libssl→libcrypto into Contents/.
	return fixSiblingInstallNames(ctx, deps, res)
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
	for _, pat := range []string{"*.so", "*.dylib"} {
		nested, err := deps.FS.Glob(filepath.Join(root, "*", "*", "loaders", pat))
		if err == nil {
			out = append(out, nested...)
		}
	}
	return out
}

func compileSchemas(ctx context.Context, deps foundation.Deps, res string) error {
	dir := filepath.Join(res, "share", "glib-2.0", "schemas")
	if _, err := deps.FS.Stat(dir); err != nil {
		return fmt.Errorf("%w: %s", ErrSchemaMissing, dir)
	}
	for _, name := range requiredSchemaFiles() {
		p := filepath.Join(dir, name)
		if _, err := deps.FS.Stat(p); err != nil {
			return fmt.Errorf("%w: %s", ErrSchemaMissing, name)
		}
	}
	if err := deps.Runner.Run(ctx, "glib-compile-schemas", dir); err != nil {
		return fmt.Errorf("%w: %w", ErrSchemaCompile, err)
	}
	compiled := filepath.Join(dir, "gschemas.compiled")
	if _, err := deps.FS.Stat(compiled); err != nil {
		return fmt.Errorf("%w: gschemas.compiled", ErrSchemaMissing)
	}
	return nil
}

const pixbufLoaderToken = "@REMMINA_LOADERS@"

func remminaSymbolicIconRel() string {
	return filepath.Join("share", "icons", "hicolor", "scalable", "actions",
		"org.remmina.Remmina-fullscreen-symbolic.svg")
}

func remminaVNCIconRel() string {
	return filepath.Join("share", "icons", "hicolor", "scalable", "emblems",
		"org.remmina.Remmina-vnc-symbolic.svg")
}

func pixbufLoadersDir(res string) string {
	return filepath.Join(res, "lib", "gdk-pixbuf-2.0", "2.10.0", "loaders")
}

func pixbufCachePath(res string) string {
	return filepath.Join(res, "lib", "gdk-pixbuf-2.0", "2.10.0", "loaders.cache")
}

func isSVGLoader(name string) bool {
	n := strings.ToLower(filepath.Base(name))
	return strings.Contains(n, "svg") && (strings.HasSuffix(n, ".so") || strings.HasSuffix(n, ".dylib"))
}

func checkRemminaIcons(deps foundation.Deps, res string) error {
	for _, rel := range []string{remminaSymbolicIconRel(), remminaVNCIconRel()} {
		if _, err := deps.FS.Stat(filepath.Join(res, rel)); err != nil {
			return fmt.Errorf("%w: %s", ErrIconMissing, rel)
		}
	}
	return nil
}

func writeGTKSettings(deps foundation.Deps, res string) error {
	dir := filepath.Join(res, "etc", "gtk-3.0")
	if err := deps.FS.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	const ini = `[Settings]
gtk-icon-theme-name=Adwaita
gtk-theme-name=Adwaita
`
	return deps.FS.WriteFile(filepath.Join(dir, "settings.ini"), []byte(ini), 0o644)
}

func updateIconCaches(ctx context.Context, deps foundation.Deps, res string) error {
	tool := "gtk-update-icon-cache"
	if _, err := deps.Runner.Output(ctx, "which", tool); err != nil {
		tool = "gtk3-update-icon-cache"
	}
	for _, theme := range []string{"hicolor", "Adwaita", "AdwaitaLegacy"} {
		dir := filepath.Join(res, "share", "icons", theme)
		if _, err := deps.FS.Stat(dir); err != nil {
			continue
		}
		if err := deps.Runner.Run(ctx, tool, "-f", "-t", dir); err != nil {
			return fmt.Errorf("gtk-update-icon-cache %s: %w", theme, err)
		}
	}
	return nil
}

func writePixbufCache(ctx context.Context, deps foundation.Deps, res, brew string) error {
	dir := pixbufLoadersDir(res)
	present := map[string]bool{}
	var hasSVG bool
	for _, pat := range []string{"*.so", "*.dylib"} {
		files, err := deps.FS.Glob(filepath.Join(dir, pat))
		if err != nil {
			return fmt.Errorf("%w: %w", ErrPixbufSVGLoader, err)
		}
		for _, p := range files {
			present[filepath.Base(p)] = true
			if isSVGLoader(p) {
				hasSVG = true
			}
		}
	}
	if !hasSVG {
		return fmt.Errorf("%w: %s", ErrPixbufSVGLoader, dir)
	}
	src, err := readBrewPixbufCache(deps, brew)
	if err != nil {
		return err
	}
	rewritten := relocatePixbufCache(src, present)
	rewritten = dropHomebrewCommentLines(rewritten)
	if !strings.Contains(strings.ToLower(rewritten), "svg") {
		return fmt.Errorf("%w: SVG loader did not register", ErrPixbufCache)
	}
	if !strings.Contains(rewritten, pixbufLoaderToken) {
		return fmt.Errorf("%w: token missing", ErrPixbufCache)
	}
	for _, n := range []string{"/opt/homebrew/", "/usr/local/opt/", "/usr/local/Cellar/"} {
		if strings.Contains(rewritten, n) {
			return fmt.Errorf("%w: still contains %s", ErrPixbufCache, n)
		}
	}
	return deps.FS.WriteFile(pixbufCachePath(res), []byte(rewritten), 0o644)
}

func readBrewPixbufCache(deps foundation.Deps, brew string) (string, error) {
	for _, p := range []string{
		filepath.Join(brew, "lib", "gdk-pixbuf-2.0", "2.10.0", "loaders.cache"),
		filepath.Join(brew, "opt", "gdk-pixbuf", "lib", "gdk-pixbuf-2.0", "2.10.0", "loaders.cache"),
	} {
		data, err := deps.FS.ReadFile(p)
		if err == nil && len(data) > 0 {
			return string(data), nil
		}
	}
	return "", fmt.Errorf("%w: brew loaders.cache missing", ErrPixbufCache)
}

func relocatePixbufCache(src string, present map[string]bool) string {
	var out, cur []string
	flush := func() {
		if len(cur) == 0 {
			return
		}
		base := loaderBaseFromPathLine(cur[0])
		if base != "" && present[base] {
			cur[0] = "\"" + pixbufLoaderToken + "/" + base + "\""
			out = append(out, cur...)
		}
		cur = nil
	}
	for _, line := range strings.Split(src, "\n") {
		if isLoaderPathLine(line) {
			flush()
			cur = []string{line}
			continue
		}
		if len(cur) > 0 {
			cur = append(cur, line)
			continue
		}
		out = append(out, line)
	}
	flush()
	return strings.Join(out, "\n") + "\n"
}

func dropHomebrewCommentLines(s string) string {
	var keep []string
	for _, line := range strings.Split(s, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "#") {
			skip := false
			for _, n := range []string{"/opt/homebrew/", "/usr/local/opt/", "/usr/local/Cellar/"} {
				if strings.Contains(line, n) {
					skip = true
					break
				}
			}
			if skip {
				continue
			}
		}
		keep = append(keep, line)
	}
	return strings.Join(keep, "\n")
}

func loaderBaseFromPathLine(line string) string {
	s := strings.TrimSpace(line)
	s = strings.Trim(s, "\"")
	if s == "" {
		return ""
	}
	return filepath.Base(s)
}

func isLoaderPathLine(line string) bool {
	s := strings.TrimSpace(line)
	return strings.HasPrefix(s, "\"/") || strings.HasPrefix(s, "\"@")
}

func rewritePixbufCacheText(src, loadersDir string, loaders []string) string {
	s := strings.ReplaceAll(src, loadersDir, pixbufLoaderToken)
	for _, p := range loaders {
		s = strings.ReplaceAll(s, p, pixbufLoaderToken+"/"+filepath.Base(p))
	}
	return s
}

func rpathDepName(dep string) string {
	const p = "@rpath/"
	if strings.HasPrefix(dep, p) {
		return strings.TrimPrefix(dep, p)
	}
	return ""
}

func findBrewDylib(deps foundation.Deps, brew, name string) string {
	for _, p := range []string{
		filepath.Join(brew, "lib", name),
		filepath.Join(brew, "opt", "librsvg", "lib", name),
	} {
		if _, err := deps.FS.Stat(p); err == nil {
			return p
		}
	}
	matches, err := deps.FS.Glob(filepath.Join(brew, "opt", "*", "lib", name))
	if err == nil && len(matches) > 0 {
		return matches[0]
	}
	return ""
}

func vendorMissingRpathLibs(ctx context.Context, deps foundation.Deps, res, brew string) error {
	lib := filepath.Join(res, "lib")
	for i := 0; i < 8; i++ {
		bundled, err := bundledLibNames(deps, lib)
		if err != nil {
			return err
		}
		needed := map[string]bool{}
		for _, f := range rpathScanFiles(deps, res) {
			out, err := deps.Runner.Output(ctx, "otool", "-L", f)
			if err != nil {
				return fmt.Errorf("otool -L %s: %w", filepath.Base(f), err)
			}
			_, depsList := parseOtoolL(out)
			for _, dep := range depsList {
				name := rpathDepName(dep)
				if name == "" || bundled[name] {
					continue
				}
				needed[name] = true
			}
		}
		if len(needed) == 0 {
			return nil
		}
		for name := range needed {
			src := findBrewDylib(deps, brew, name)
			if src == "" {
				return fmt.Errorf("%w: %s", ErrRpathLib, name)
			}
			dest := filepath.Join(lib, name)
			deps.Logf("vendor @rpath %s from %s", name, src)
			if err := deps.Runner.Run(ctx, "cp", src, dest); err != nil {
				return fmt.Errorf("copy %s: %w", name, err)
			}
			_ = deps.Runner.Run(ctx, "chmod", "u+w", dest)
			args := dylibbundlerArgs(dest, lib, "@loader_path/")
			if err := deps.Runner.Run(ctx, "dylibbundler", args...); err != nil {
				return fmt.Errorf("%w: %s: %w", ErrDylibbundler, name, err)
			}
		}
	}
	return fmt.Errorf("%w: too many @rpath rounds", ErrRpathLib)
}

func rpathScanFiles(deps foundation.Deps, res string) []string {
	var out []string
	out = append(out, filepath.Join(res, "bin", "remmina"))
	out = append(out, globMachO(deps, filepath.Join(res, "lib", "remmina", "plugins"))...)
	out = append(out, globMachO(deps, filepath.Join(res, "lib", "gdk-pixbuf-2.0"))...)
	matches, err := deps.FS.Glob(filepath.Join(res, "lib", "*"))
	if err != nil {
		return out
	}
	for _, p := range matches {
		base := filepath.Base(p)
		if strings.HasSuffix(base, ".dylib") || strings.HasSuffix(base, ".so") {
			out = append(out, p)
		}
	}
	return out
}

func fixLoaderInstallNames(ctx context.Context, deps foundation.Deps, res string) error {
	bundled, err := bundledLibNames(deps, filepath.Join(res, "lib"))
	if err != nil {
		return err
	}
	prefix := loaderPathToLib(res, filepath.Join(pixbufLoadersDir(res), "x.so"))
	if !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}
	for _, pat := range []string{"*.so", "*.dylib"} {
		files, err := deps.FS.Glob(filepath.Join(pixbufLoadersDir(res), pat))
		if err != nil {
			return err
		}
		for _, f := range files {
			if err := rewriteInstallNames(ctx, deps, f, prefix, bundled); err != nil {
				return err
			}
		}
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
