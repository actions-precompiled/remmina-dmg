package main

import "testing"

func TestParseOtoolL(t *testing.T) {
	t.Parallel()
	out := `libssl.3.dylib:
	@loader_path/../lib/libssl.3.dylib (compatibility version 3.0.0, current version 3.0.0)
	@loader_path/../../libcrypto.3.dylib (compatibility version 3.0.0, current version 3.0.0)
	/usr/lib/libSystem.B.dylib (compatibility version 1.0.0, current version 1356.0.0)
`
	id, deps := parseOtoolL(out)
	if id != "@loader_path/../lib/libssl.3.dylib" {
		t.Fatalf("id %q", id)
	}
	if len(deps) != 2 || deps[0] != "@loader_path/../../libcrypto.3.dylib" {
		t.Fatalf("deps %v", deps)
	}
}

func TestIsForeignInstallName(t *testing.T) {
	t.Parallel()
	if !isForeignInstallName("/opt/homebrew/opt/librsvg/lib/gdk-pixbuf-2.0/2.10.0/loaders/libpixbufloader_svg.dylib") {
		t.Fatal("homebrew id")
	}
	if isForeignInstallName("@loader_path/libpixbufloader_svg.dylib") {
		t.Fatal("loader_path is local")
	}
}

func TestLibHasBadSiblingPath(t *testing.T) {
	t.Parallel()
	bad := `libssl.3.dylib:
	@loader_path/libssl.3.dylib (compatibility version 3.0.0, current version 3.0.0)
	@loader_path/../../libcrypto.3.dylib (compatibility version 3.0.0, current version 3.0.0)
`
	if !libHasBadSiblingPath(bad) {
		t.Fatal("../../ should be bad for a lib/ dylib")
	}
	good := `libssl.3.dylib:
	@loader_path/libssl.3.dylib (compatibility version 3.0.0, current version 3.0.0)
	@loader_path/libcrypto.3.dylib (compatibility version 3.0.0, current version 3.0.0)
	/usr/lib/libSystem.B.dylib (compatibility version 1.0.0, current version 1356.0.0)
`
	if libHasBadSiblingPath(good) {
		t.Fatal("sibling @loader_path/ is ok")
	}
}
