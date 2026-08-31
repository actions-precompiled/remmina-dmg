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

	app := filepath.Join(mount, "Remmina.app")
	bin := filepath.Join(app, "Contents", "Resources", "bin", "remmina")
	if _, err := deps.FS.Stat(bin); err != nil {
		return fmt.Errorf("%w: %s", ErrSmokeAppMissing, bin)
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

	env := foundation.CleanSmokeEnv(deps.Env.Environ())
	out, err := foundation.OutputWithEnv(ctx, deps, env, bin, "--version")
	if err != nil {
		return fmt.Errorf("remmina --version: %w\n%s", err, out)
	}
	if !strings.Contains(strings.ToLower(out), "remmina") {
		return fmt.Errorf("remmina --version: unexpected output: %s", strings.TrimSpace(out))
	}
	deps.Logf("remmina --version: %s", firstLine(out))
	deps.Logf("✓ Smoke test passed: %s", filepath.Base(artifact))
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
