package main

import (
	"context"
	"fmt"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/actions-precompiled/foundation"
)

func smokeArtifacts(ctx context.Context, deps foundation.Deps, meta foundation.Meta, req foundation.SmokeRequest) error {
	if len(req.Tarballs) == 0 {
		return fmt.Errorf("%w", ErrSmokeNoArtifacts)
	}
	for _, art := range req.Tarballs {
		if err := smokeOne(ctx, deps, art); err != nil {
			return err
		}
	}
	return nil
}

func smokeOne(ctx context.Context, deps foundation.Deps, artifact string) error {
	deps.Logf("Smoke test: %s", filepath.Base(artifact))
	if !strings.HasSuffix(strings.ToLower(artifact), ".dmg") {
		return fmt.Errorf("%w: %s", ErrSmokeNoArtifacts, artifact)
	}
	if runtime.GOOS != "darwin" {
		deps.Logf("smoke: not on macOS; checking artifact exists only")
		if _, err := deps.FS.Stat(artifact); err != nil {
			return fmt.Errorf("%w: %s", ErrSmokeNoArtifacts, artifact)
		}
		return nil
	}

	tmp, err := deps.FS.TempDir("", "remmina-smoke-")
	if err != nil {
		return err
	}
	defer deps.RemoveAllLog(tmp, "smoke cleanup")

	mount := filepath.Join(tmp, "mnt")
	if err := deps.FS.MkdirAll(mount, 0o755); err != nil {
		return err
	}
	if err := deps.Runner.Run(ctx, "hdiutil", "attach", artifact, "-nobrowse", "-readonly", "-mountpoint", mount); err != nil {
		return fmt.Errorf("hdiutil attach: %w", err)
	}
	defer func() {
		_ = deps.Runner.Run(ctx, "hdiutil", "detach", mount, "-quiet")
	}()

	mounted := filepath.Join(mount, "Remmina.app")
	app := filepath.Join(tmp, "Remmina.app")
	if err := deps.Runner.Run(ctx, "cp", "-a", mounted, app); err != nil {
		return fmt.Errorf("copy app off dmg: %w", err)
	}

	bin := filepath.Join(app, "Contents", "Resources", "bin", "remmina")
	launcher := filepath.Join(app, "Contents", "MacOS", "Remmina")
	if _, err := deps.FS.Stat(bin); err != nil {
		return fmt.Errorf("%w: %s", ErrSmokeAppMissing, bin)
	}
	if _, err := deps.FS.Stat(launcher); err != nil {
		return fmt.Errorf("%w: %s", ErrSmokeAppMissing, launcher)
	}
	icns := filepath.Join(app, "Contents", "Resources", "AppIcon.icns")
	if _, err := deps.FS.Stat(icns); err != nil {
		return fmt.Errorf("%w: %s", ErrAppIconICNS, icns)
	}
	for _, name := range requiredPlugins {
		p := filepath.Join(app, "Contents", "Resources", "lib", "remmina", "plugins", name)
		if _, err := deps.FS.Stat(p); err != nil {
			return fmt.Errorf("%w: %s", ErrPluginMissing, name)
		}
	}

	if err := checkNoHomebrewLinks(ctx, deps, bin); err != nil {
		return err
	}
	rdp := filepath.Join(app, "Contents", "Resources", "lib", "remmina", "plugins", "remmina-plugin-rdp.so")
	if err := checkNoHomebrewLinks(ctx, deps, rdp); err != nil {
		return err
	}
	ssl := filepath.Join(app, "Contents", "Resources", "lib", "libssl.3.dylib")
	if err := checkLibSiblings(ctx, deps, ssl); err != nil {
		return err
	}
	if err := checkSchemas(deps, app); err != nil {
		return err
	}
	if err := checkIcons(deps, app); err != nil {
		return err
	}
	if err := checkSVGLoader(ctx, deps, app); err != nil {
		return err
	}
	if err := verifyAdHoc(ctx, deps, app); err != nil {
		return err
	}
	if err := verifyAdHoc(ctx, deps, bin); err != nil {
		return err
	}

	env := foundation.CleanSmokeEnv(deps.Env.Environ())
	out, err := foundation.OutputWithEnv(ctx, deps, env, launcher, "--version")
	if err != nil {
		if isMissingDylib(err, out) {
			return fmt.Errorf("remmina --version: %w\n%s", err, out)
		}
		// GHA macos runners have no Aqua session; GTK Quartz often SIGABRT
		// after dyld has already resolved everything.
		deps.Logf("remmina --version: %v (headless GTK; otool was clean)", err)
		if strings.TrimSpace(out) != "" {
			deps.Logf("%s", strings.TrimSpace(out))
		}
	} else if !strings.Contains(strings.ToLower(out), "remmina") {
		return fmt.Errorf("remmina --version: unexpected output: %s", strings.TrimSpace(out))
	} else {
		deps.Logf("remmina --version: %s", firstLine(out))
	}
	deps.Logf("✓ Smoke test passed: %s", filepath.Base(artifact))
	return nil
}

func verifyAdHoc(ctx context.Context, deps foundation.Deps, path string) error {
	out, err := deps.Runner.Output(ctx, "codesign", "--verify", "--verbose=2", path)
	if err != nil {
		return fmt.Errorf("codesign --verify %s: %w\n%s", filepath.Base(path), err, out)
	}
	return nil
}

func isMissingDylib(err error, out string) bool {
	blob := strings.ToLower(err.Error() + "\n" + out)
	for _, n := range []string{
		"library not loaded",
		"image not found",
		"dyld[",
		"symbol not found",
	} {
		if strings.Contains(blob, n) {
			return true
		}
	}
	return false
}

func checkIcons(deps foundation.Deps, app string) error {
	res := filepath.Join(app, "Contents", "Resources")
	if err := checkRemminaIcons(deps, res); err != nil {
		return err
	}
	loaders, err := deps.FS.Glob(filepath.Join(pixbufLoadersDir(res), "*.so"))
	if err != nil {
		return fmt.Errorf("%w: %w", ErrPixbufSVGLoader, err)
	}
	ok := false
	for _, p := range loaders {
		if isSVGLoader(p) {
			ok = true
			break
		}
	}
	if !ok {
		return fmt.Errorf("%w", ErrPixbufSVGLoader)
	}
	cache, err := deps.FS.ReadFile(pixbufCachePath(res))
	if err != nil {
		return fmt.Errorf("%w: loaders.cache: %w", ErrPixbufCache, err)
	}
	s := string(cache)
	if !strings.Contains(s, pixbufLoaderToken) {
		return fmt.Errorf("%w: token missing", ErrPixbufCache)
	}
	if !strings.Contains(strings.ToLower(s), "svg") {
		return fmt.Errorf("%w: SVG loader did not register", ErrPixbufCache)
	}
	if !strings.Contains(s, "\n\n") {
		return fmt.Errorf("%w: module must end with a blank line", ErrPixbufCache)
	}
	for _, n := range []string{"/opt/homebrew/", "/usr/local/opt/", "/usr/local/Cellar/"} {
		if strings.Contains(s, n) {
			return fmt.Errorf("%w: still contains %s", ErrPixbufCache, n)
		}
	}
	return nil
}

func checkSVGLoader(ctx context.Context, deps foundation.Deps, app string) error {
	res := filepath.Join(app, "Contents", "Resources")
	if _, err := deps.FS.Stat(filepath.Join(res, "etc", "gtk-3.0", "settings.ini")); err != nil {
		return fmt.Errorf("%w: gtk-3.0/settings.ini", ErrIconMissing)
	}
	rsvg, err := deps.FS.Glob(filepath.Join(res, "lib", "librsvg*.dylib"))
	if err != nil || len(rsvg) == 0 {
		return fmt.Errorf("%w: librsvg*.dylib", ErrRpathLib)
	}
	var loader string
	files, err := deps.FS.Glob(filepath.Join(pixbufLoadersDir(res), "*"))
	if err != nil {
		return fmt.Errorf("%w: %w", ErrPixbufSVGLoader, err)
	}
	for _, p := range files {
		if isSVGLoader(p) {
			loader = p
			break
		}
	}
	if loader == "" {
		return fmt.Errorf("%w", ErrPixbufSVGLoader)
	}
	if err := checkNoHomebrewLinks(ctx, deps, loader); err != nil {
		return err
	}
	out, err := deps.Runner.Output(ctx, "otool", "-L", loader)
	if err != nil {
		return fmt.Errorf("otool -L %s: %w", filepath.Base(loader), err)
	}
	if strings.Contains(out, "@rpath/librsvg") {
		return fmt.Errorf("%w: %s still uses @rpath/librsvg", ErrRpathLib, filepath.Base(loader))
	}
	if !strings.Contains(out, "librsvg") {
		return fmt.Errorf("%w: %s does not link librsvg", ErrPixbufSVGLoader, filepath.Base(loader))
	}
	return nil
}

func checkSchemas(deps foundation.Deps, app string) error {
	dir := filepath.Join(app, "Contents", "Resources", "share", "glib-2.0", "schemas")
	for _, name := range requiredSchemaFiles() {
		if _, err := deps.FS.Stat(filepath.Join(dir, name)); err != nil {
			return fmt.Errorf("%w: %s", ErrSchemaMissing, name)
		}
	}
	if _, err := deps.FS.Stat(filepath.Join(dir, "gschemas.compiled")); err != nil {
		return fmt.Errorf("%w: gschemas.compiled", ErrSchemaMissing)
	}
	return nil
}

func checkLibSiblings(ctx context.Context, deps foundation.Deps, lib string) error {
	out, err := deps.Runner.Output(ctx, "otool", "-L", lib)
	if err != nil {
		return fmt.Errorf("otool -L %s: %w", filepath.Base(lib), err)
	}
	if libHasBadSiblingPath(out) {
		return fmt.Errorf("%w: %s\n%s", ErrBadInstallName, filepath.Base(lib), out)
	}
	return nil
}

func checkNoHomebrewLinks(ctx context.Context, deps foundation.Deps, bin string) error {
	out, err := deps.Runner.Output(ctx, "otool", "-L", bin)
	if err != nil {
		return fmt.Errorf("otool -L: %w", err)
	}
	for line := range strings.SplitSeq(out, "\n") {
		s := strings.TrimSpace(line)
		if s == "" {
			continue
		}
		if strings.Contains(s, "/opt/homebrew/") || strings.Contains(s, "/usr/local/opt/") || strings.Contains(s, "/usr/local/Cellar/") {
			return fmt.Errorf("%w: %s", ErrHomebrewLink, s)
		}
	}
	return nil
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return strings.TrimSpace(s[:i])
	}
	return s
}
