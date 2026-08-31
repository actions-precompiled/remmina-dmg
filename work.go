package main

import (
	"context"
	"fmt"
	"path/filepath"
	"runtime"
	"strconv"
	"time"

	"github.com/actions-precompiled/foundation"
)

func workRemmina(ctx context.Context, deps foundation.Deps, meta foundation.Meta, req foundation.BuildRequest) error {
	if runtime.GOOS != "darwin" {
		return fmt.Errorf("%w", ErrNeedDarwin)
	}
	target, err := normalizeDarwinTarget(req.Target)
	if err != nil {
		return err
	}
	if target != foundation.HostTarget() {
		return fmt.Errorf("%w: want %s on this host, got %s", ErrHostMismatch, foundation.HostTarget(), target)
	}

	jobs := strconv.Itoa(runtime.NumCPU())
	work := buildWorkRoot(deps, meta)
	src := filepath.Join(work, "src")
	build := filepath.Join(work, "build")
	stage := filepath.Join(work, "stage")
	prefix := filepath.Join(stage, "Resources")

	deps.RemoveAllLog(build, "remove")
	deps.RemoveAllLog(stage, "remove")
	for _, d := range []string{build, prefix, req.OutDir} {
		if err := deps.FS.MkdirAll(d, 0o755); err != nil {
			return err
		}
	}

	src, ref, artifactVer, sha, err := resolveSource(ctx, deps, meta, req.Version, src)
	if err != nil {
		return err
	}
	deps.Logf("Resolved ref=%s sha=%s artifact=%s src=%s", ref, sha, artifactVer, src)

	if err := injectBundleSources(deps, src); err != nil {
		return err
	}

	brew, err := brewPrefix(ctx, deps)
	if err != nil {
		return err
	}

	cmakeArgs := remminaCMakeArgs(src, build, prefix, brew)
	if err := deps.Runner.Run(ctx, "cmake", cmakeArgs...); err != nil {
		return fmt.Errorf("cmake configure: %w", err)
	}
	if err := deps.Runner.Run(ctx, "cmake", "--build", build, "--parallel", jobs); err != nil {
		return fmt.Errorf("cmake build: %w", err)
	}
	if err := deps.Runner.Run(ctx, "cmake", "--install", build); err != nil {
		return fmt.Errorf("cmake install: %w", err)
	}

	bin := filepath.Join(prefix, "bin", "remmina")
	if _, err := deps.FS.Stat(bin); err != nil {
		return fmt.Errorf("%w: %s", ErrRemminaMissing, bin)
	}
	pluginDir := filepath.Join(prefix, "lib", "remmina", "plugins")
	for _, name := range requiredPlugins {
		if _, err := deps.FS.Stat(filepath.Join(pluginDir, name)); err != nil {
			return fmt.Errorf("%w: %s", ErrPluginMissing, name)
		}
	}

	appDir := filepath.Join(stage, "Remmina.app")
	if err := assembleApp(ctx, deps, prefix, appDir, brew, artifactVer); err != nil {
		return err
	}

	info := fmt.Sprintf(`package=%s
version=%s
upstream_ref=%s
upstream_sha=%s
build_target=%s
distributor=actions-precompiled
signed=ad-hoc
built_at=%s
`, meta.Name, artifactVer, ref, sha, target, time.Now().UTC().Format(time.RFC3339))
	if err := deps.FS.WriteFile(filepath.Join(appDir, "Contents", "Resources", "BUILDINFO.txt"), []byte(info), 0o644); err != nil {
		return err
	}
	if err := signApp(ctx, deps, appDir); err != nil {
		return err
	}

	dmg := filepath.Join(req.OutDir, foundation.ArtifactNameExt(meta.Name, artifactVer, target, ".dmg"))
	if err := writeDMG(ctx, deps, appDir, dmg); err != nil {
		return err
	}
	deps.Logf("Done: %s", dmg)
	return nil
}

func normalizeDarwinTarget(target string) (string, error) {
	switch target {
	case foundation.TargetDarwinAArch64, "darwin-arm64", "macos-arm64", "macos-aarch64":
		return foundation.TargetDarwinAArch64, nil
	case foundation.TargetDarwinAMD64, "darwin-x86_64", "macos-amd64", "macos-x86_64":
		return foundation.TargetDarwinAMD64, nil
	default:
		return "", fmt.Errorf("%w: %q", ErrUnsupportedTarget, target)
	}
}
