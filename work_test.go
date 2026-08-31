package main

import (
	"errors"
	"runtime"
	"testing"

	"github.com/actions-precompiled/foundation"
)

func TestNormalizeDarwinTarget(t *testing.T) {
	t.Parallel()
	got, err := normalizeDarwinTarget("macos-arm64")
	if err != nil || got != foundation.TargetDarwinAArch64 {
		t.Fatalf("got %q %v", got, err)
	}
	got, err = normalizeDarwinTarget(foundation.TargetDarwinAMD64)
	if err != nil || got != foundation.TargetDarwinAMD64 {
		t.Fatalf("intel alias: %q %v", got, err)
	}
	if _, err := normalizeDarwinTarget("linux-amd64"); !errors.Is(err, ErrUnsupportedTarget) {
		t.Fatalf("want ErrUnsupportedTarget, got %v", err)
	}
}

func TestWorkRefusesNonDarwinHost(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "darwin" {
		t.Skip("this host is macOS")
	}
	err := workRemmina(t.Context(), foundation.Deps{}, foundation.Meta{Name: "remmina"}, foundation.BuildRequest{
		Version: "v1.4.43",
		Target:  foundation.TargetDarwinAArch64,
		OutDir:  t.TempDir(),
	})
	if !errors.Is(err, ErrNeedDarwin) {
		t.Fatalf("got %v", err)
	}
}
