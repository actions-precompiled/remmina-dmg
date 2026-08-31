package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/actions-precompiled/foundation"
)

var brewDeps = []string{
	"cmake",
	"ninja",
	"pkg-config",
	"gettext",
	"gtk+3",
	"freerdp",
	"libvncserver",
	"libssh",
	"json-glib",
	"libsodium",
	"libgcrypt",
	"openssl@3",
	"adwaita-icon-theme",
	"hicolor-icon-theme",
	"gdk-pixbuf",
	"librsvg",
	"glib",
	"dylibbundler",
}

func (remminaPackage) PrepHost(ctx context.Context, deps foundation.Deps, cfg foundation.Config) error {
	return prepHostTools(ctx, deps)
}

func prepHostTools(ctx context.Context, deps foundation.Deps) error {
	if _, err := deps.Runner.Output(ctx, "brew", "--prefix"); err != nil {
		return fmt.Errorf("%w: brew", ErrHostToolMissing)
	}
	args := append([]string{"install", "--quiet"}, brewDeps...)
	deps.Logf("PrepHost: brew install %s", strings.Join(brewDeps, " "))
	if err := deps.Runner.Run(ctx, "brew", args...); err != nil {
		return fmt.Errorf("%w: brew install: %w", ErrHostToolMissing, err)
	}
	for _, tool := range []string{"cmake", "ninja", "pkg-config", "git", "hdiutil", "dylibbundler", "codesign"} {
		if _, err := deps.Runner.Output(ctx, "which", tool); err != nil {
			return fmt.Errorf("%w: %s", ErrHostToolMissing, tool)
		}
	}
	return nil
}

func brewPrefix(ctx context.Context, deps foundation.Deps) (string, error) {
	out, err := deps.Runner.Output(ctx, "brew", "--prefix")
	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrBrewPrefix, err)
	}
	p := strings.TrimSpace(out)
	if p == "" {
		return "", fmt.Errorf("%w", ErrBrewPrefix)
	}
	return p, nil
}
