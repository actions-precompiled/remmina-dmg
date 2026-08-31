// Command apc builds unsigned Remmina DMGs for macOS.
//
//	go run . list
//	go run . build v1.4.43
//	go run . generate workflow --force
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/actions-precompiled/foundation"
)

func main() {
	deps := foundation.DefaultDeps(".")
	deps.GitHub = newGitLabTags(deps)
	if err := foundation.MainWith(remminaPackage{}, deps, os.Args[1:]); err != nil {
		code := 1
		var ee *foundation.ExitError
		if foundation.AsExitError(err, &ee) {
			code = ee.Code
			if ee.Err != nil {
				fmt.Fprintln(os.Stderr, ee.Err)
			}
		} else {
			fmt.Fprintln(os.Stderr, err)
		}
		os.Exit(code)
	}
}

type remminaPackage struct{}

func (remminaPackage) Meta() foundation.Meta {
	return foundation.Meta{
		Name:            "remmina",
		UpstreamRepoAPI: "Remmina/Remmina",
		UpstreamGit:     "https://gitlab.com/Remmina/Remmina.git",
		Binary:          "remmina",
		VersionEnv:      "REMMINA_VERSION",
		Description:     "Unsigned self-contained Remmina.app in a DMG (GTK remote desktop client).",
		Homepage:        "https://remmina.org/",
		HostOnly:        true,
		DefaultTargets:  []string{foundation.TargetDarwinAArch64},
	}
}

func (p remminaPackage) Work(ctx context.Context, deps foundation.Deps, req foundation.BuildRequest) error {
	return workRemmina(ctx, deps, p.Meta().Normalize(), req)
}

func (p remminaPackage) Smoke(ctx context.Context, deps foundation.Deps, req foundation.SmokeRequest) error {
	return smokeArtifacts(ctx, deps, p.Meta().Normalize(), req)
}
