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
		"CFBundleIconFile",
		"AppIcon",
	} {
		if !strings.Contains(plist, want) {
			t.Fatalf("plist missing %q", want)
		}
	}
	if !strings.Contains(plist, "CFBundleIconFile") {
		t.Fatal("plist must name the app icon")
	}
}

func TestDylibbundlerDoesNotWipeDest(t *testing.T) {
	t.Parallel()
	// Regression: -od deletes lib/remmina/plugins before plugins are bundled.
	for _, a := range dylibbundlerArgs("x", "d", "p") {
		if a == "-od" {
			t.Fatal("dylibbundler must not use -od")
		}
	}
}

func TestIconsetSlots(t *testing.T) {
	t.Parallel()
	got := iconsetSlots()
	if len(got) < 9 {
		t.Fatalf("too few slots: %d", len(got))
	}
	want := map[string]bool{
		"icon_16x16.png": false, "icon_512x512.png": false,
	}
	for _, s := range got {
		if _, ok := want[s.name]; ok {
			want[s.name] = true
		}
	}
	for name, seen := range want {
		if !seen {
			t.Fatalf("missing slot %s", name)
		}
	}
}

func TestMergeTreeCopiesContents(t *testing.T) {
	t.Parallel()
	src := "/opt/homebrew/share/glib-2.0/schemas"
	dest := "/tmp/Remmina.app/Contents/Resources/share/glib-2.0/schemas"
	args := mergeTreeArgs(src, dest)
	if len(args) != 3 || args[0] != "-RL" || args[1] != src+"/." || args[2] != dest {
		t.Fatalf("merge args: %v", args)
	}
	for _, a := range args {
		if a == "-a" || a == "-R" {
			t.Fatal("cp -a/-R of the directory itself nests when dest exists")
		}
	}
}

func TestRequiredSchemasIncludeFileChooser(t *testing.T) {
	t.Parallel()
	got := strings.Join(requiredSchemaFiles(), " ")
	if !strings.Contains(got, "org.gtk.Settings.FileChooser.gschema.xml") {
		t.Fatal("GTK FileChooser schema required; missing it SIGTRAPs on connect")
	}
}

func TestPixbufCacheIsRelocatable(t *testing.T) {
	t.Parallel()
	dir := "/tmp/Remmina.app/Contents/Resources/lib/gdk-pixbuf-2.0/2.10.0/loaders"
	svg := dir + "/libpixbufloader-svg.so"
	src := "\"" + svg + "\"\n\"png\" 5\n"
	got := rewritePixbufCacheText(src, dir, []string{svg})
	if !strings.Contains(got, pixbufLoaderToken+"/libpixbufloader-svg.so") {
		t.Fatalf("token path: %s", got)
	}
	if strings.Contains(got, "/tmp/Remmina.app") {
		t.Fatalf("absolute path left: %s", got)
	}
}

func TestSVGLoaderName(t *testing.T) {
	t.Parallel()
	if !isSVGLoader("libpixbufloader-svg.so") || !isSVGLoader("libpixbufloader_svg.so") {
		t.Fatal("svg loader names")
	}
	if isSVGLoader("libpixbufloader-png.so") {
		t.Fatal("png is not svg")
	}
}

func TestRemminaToolbarIconIsSVG(t *testing.T) {
	t.Parallel()
	rel := remminaSymbolicIconRel()
	if !strings.Contains(rel, "org.remmina.Remmina-fullscreen-symbolic.svg") {
		t.Fatalf("toolbar icons are remmina SVGs: %s", rel)
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
