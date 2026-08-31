package main

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/actions-precompiled/foundation"
)

func cloneUpstream(ctx context.Context, deps foundation.Deps, upstream, versionRaw, src string) (ref, artifact, sha string, err error) {
	tryClone := func(branch string) error {
		deps.RemoveAllLog(src, "remove")
		return deps.Runner.Run(ctx, "git", "clone", "--depth", "1", "--branch", branch, upstream, src)
	}

	ref = versionRaw
	if !strings.HasPrefix(ref, "v") && foundation.ParseVersion(ref).Parts != nil {
		ref = "v" + strings.TrimPrefix(versionRaw, "v")
	}
	if err := tryClone(ref); err != nil {
		if ref != versionRaw {
			if err2 := tryClone(versionRaw); err2 == nil {
				ref = versionRaw
			} else {
				return "", "", "", fmt.Errorf("%w: %s: %w", ErrCloneFailed, versionRaw, err)
			}
		} else {
			return "", "", "", fmt.Errorf("%w: %w", ErrCloneFailed, err)
		}
	}

	out, err := deps.Runner.Output(ctx, "git", "-C", src, "rev-parse", "--short=12", "HEAD")
	if err != nil {
		return "", "", "", err
	}
	sha = strings.TrimSpace(out)
	artifact = foundation.VersionBare(versionRaw)
	return ref, artifact, sha, nil
}

func resolveSource(ctx context.Context, deps foundation.Deps, meta foundation.Meta, version, fallbackSrc string) (src, ref, artifact, sha string, err error) {
	if pre := deps.Env.Get("APC_PREBUILT_SRC"); pre != "" {
		if st, e := deps.FS.Stat(pre); e == nil && st.IsDir() {
			ref, artifact, sha, err = gitIdentity(ctx, deps, pre, version)
			if err != nil {
				return "", "", "", "", err
			}
			deps.Logf("source: using prebuilt mount %s", pre)
			return pre, ref, artifact, sha, nil
		}
	}
	ref, artifact, sha, err = cloneUpstream(ctx, deps, meta.UpstreamGit, version, fallbackSrc)
	return fallbackSrc, ref, artifact, sha, err
}

func gitIdentity(ctx context.Context, deps foundation.Deps, src, versionRaw string) (ref, artifact, sha string, err error) {
	out, err := deps.Runner.Output(ctx, "git", "-C", src, "rev-parse", "--short=12", "HEAD")
	if err != nil {
		return "", "", "", err
	}
	sha = strings.TrimSpace(out)
	ref = versionRaw
	artifact = foundation.VersionBare(versionRaw)
	return ref, artifact, sha, nil
}

func buildWorkRoot(deps foundation.Deps, meta foundation.Meta) string {
	if w := deps.Env.Get("APC_WORK_ROOT"); w != "" {
		return filepath.Join(w, meta.Name+"-build")
	}
	return filepath.Join("/tmp", meta.Name+"-build")
}
