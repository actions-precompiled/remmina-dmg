package main

import (
	"errors"
	"testing"
)

func TestIsMissingDylib(t *testing.T) {
	t.Parallel()
	if !isMissingDylib(errors.New("exit 1"), "dyld[12]: Library not loaded: @loader_path/../lib/libgtk-3.0.dylib") {
		t.Fatal("dyld missing lib")
	}
	if isMissingDylib(errors.New("signal: abort trap"), "") {
		t.Fatal("headless abort is not a missing dylib")
	}
}
