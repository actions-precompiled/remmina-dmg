package main

import (
	"strings"
	"testing"
)

func TestRemminaCMakeArgs(t *testing.T) {
	t.Parallel()
	args := remminaCMakeArgs("/src", "/build", "/prefix", "/opt/homebrew")
	joined := strings.Join(args, " ")
	for _, want := range []string{
		"-DWITH_FREERDP3=ON",
		"-DHAVE_LIBAPPINDICATOR=OFF",
		"-DWITH_WWW=OFF",
		"-DWITH_PYTHONLIBS=OFF",
		"-DWITH_VTE=OFF",
		"-DCMAKE_PREFIX_PATH=/opt/homebrew",
		"-DCMAKE_C_FLAGS=-I/opt/homebrew/include",
		"-S /src",
		"-B /build",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("missing %q in %s", want, joined)
		}
	}
	if strings.Contains(joined, "WITH_SPICE=ON") {
		t.Fatal("spice should stay auto-off unless brew has it")
	}
}
