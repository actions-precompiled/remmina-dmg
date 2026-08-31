package main

import (
	"strings"
	"testing"
)

func TestInfoPlistAndLauncher(t *testing.T) {
	t.Parallel()
	plist := sprintfInfo("1.4.43")
	for _, want := range []string{
		"org.remmina.Remmina",
		"<string>Remmina</string>",
		"<string>1.4.43</string>",
		"CFBundleExecutable",
	} {
		if !strings.Contains(plist, want) {
			t.Fatalf("plist missing %q", want)
		}
	}
	if !strings.Contains(launcherScript, `exec "$RES/bin/remmina"`) {
		t.Fatal("launcher must exec bundled remmina")
	}
	if strings.Contains(launcherScript, "DYLD_LIBRARY_PATH") {
		t.Fatal("do not rely on DYLD_LIBRARY_PATH")
	}
}

func TestLoaderPathToLib(t *testing.T) {
	t.Parallel()
	res := "/tmp/stage/Remmina.app/Contents/Resources"
	got := loaderPathToLib(res, res+"/bin/remmina")
	if got != "@loader_path/../lib" {
		t.Fatalf("bin: %s", got)
	}
	got = loaderPathToLib(res, res+"/lib/remmina/plugins/remmina-plugin-rdp.so")
	if got != "@loader_path/../.." {
		t.Fatalf("plugin: %s", got)
	}
	got = loaderPathToLib(res, res+"/lib/gdk-pixbuf-2.0/2.10.0/loaders/libpixbufloader-png.so")
	if got != "@loader_path/../../../" && got != "@loader_path/../../.." {
		t.Fatalf("loader: %s", got)
	}
}

func sprintfInfo(ver string) string {
	return strings.ReplaceAll(infoPlist, "%s", ver)
}
