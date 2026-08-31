package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/actions-precompiled/foundation"
)

func TestReplaceOnce(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	p := filepath.Join(dir, "f.c")
	if err := os.WriteFile(p, []byte("aaa X bbb\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	deps := foundation.Deps{FS: foundation.OSFileSystem{}}
	if err := replaceOnce(deps, p, "X", "Y"); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "aaa Y bbb\n" {
		t.Fatalf("got %q", got)
	}
	if err := replaceOnce(deps, p, "X", "Z"); err == nil {
		t.Fatal("expected missing pattern")
	}
}

func TestInjectBundleSources(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	srcDir := filepath.Join(root, "src")
	if err := os.MkdirAll(srcDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "CMakeLists.txt"), []byte(`if(NOT FREEBSD)
  set(CMAKE_INSTALL_RPATH "\$ORIGIN/../${CMAKE_INSTALL_LIBDIR}:\$ORIGIN/..")
endif()
`), 0o644); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"CMakeLists.txt":           "  \"remmina_utils.c\"\n  \"remmina_utils.h\"\n  \"remmina_widget_pool.c\"\n",
		"remmina.c":                "#include \"remmina_public.h\"\nfoo\ngtk_icon_theme_append_search_path(gtk_icon_theme_get_default(),\n  REMMINA_RUNTIME_DATADIR G_DIR_SEPARATOR_S \"icons\");\nbar\nbindtextdomain(GETTEXT_PACKAGE, REMMINA_RUNTIME_LOCALEDIR);\n",
		"remmina_plugin_manager.c": "#include \"remmina_utils.h\"\ng_ptr_array_add(plugin_dirs, REMMINA_RUNTIME_PLUGINDIR);\n",
		"remmina_public.c":         "#include \"config.h\"\ngchar *ui_path = g_strconcat(REMMINA_RUNTIME_UIDIR, G_DIR_SEPARATOR_S, filename, NULL);\n",
		"remmina_external_tools.c": "#include \"remmina_external_tools.h\"\nstrcpy(dirname, REMMINA_RUNTIME_EXTERNAL_TOOLS_DIR);\ng_snprintf(launcher, MAX_PATH_LEN, \"%s/launcher.sh\", REMMINA_RUNTIME_EXTERNAL_TOOLS_DIR);\n",
		"remmina_pref_dialog.c":    "#include \"remmina_public.h\"\ngtk_file_chooser_set_current_folder(GTK_FILE_CHOOSER(remmina_pref_dialog->button_term_cs), REMMINA_RUNTIME_TERM_CS_DIR);\n",
		"remmina_main.c":           "#include \"remmina_unlock.h\"\nremminamain->window = GTK_WINDOW(RM_GET_OBJECT(\"RemminaMain\"));\n",
		"remmina_pref.c":           "	if (!keymap || keymap[0] == '\\0')\n		return keyval;\n",
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(srcDir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	deps := foundation.Deps{FS: foundation.OSFileSystem{}, Stderr: os.Stderr}
	if err := injectBundleSources(deps, root); err != nil {
		t.Fatal(err)
	}
	c, err := os.ReadFile(filepath.Join(srcDir, "remmina.c"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(c)
	if !strings.Contains(s, "remmina_bundle.h") || !strings.Contains(s, "remmina_runtime_iconsdir()") || !strings.Contains(s, "remmina_macos_init()") {
		t.Fatalf("remmina.c not injected:\n%s", s)
	}
	if _, err := os.Stat(filepath.Join(srcDir, "remmina_bundle.c")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(srcDir, "remmina_macos.c")); err != nil {
		t.Fatal(err)
	}
	rootGot, err := os.ReadFile(filepath.Join(root, "CMakeLists.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(rootGot), `set(CMAKE_INSTALL_RPATH "@loader_path/../lib")`) {
		t.Fatalf("root CMakeLists RPATH not patched:\n%s", rootGot)
	}
	if err := injectBundleSources(deps, root); err != nil {
		t.Fatal(err)
	}
}

func TestInjectOnUpstreamTree(t *testing.T) {
	src := os.Getenv("REMMINA_SRC")
	if src == "" {
		t.Skip("set REMMINA_SRC to an unpacked Remmina tree")
	}
	deps := foundation.Deps{FS: foundation.OSFileSystem{}, Stderr: os.Stderr}
	if err := injectBundleSources(deps, src); err != nil {
		t.Fatal(err)
	}
	c, err := os.ReadFile(filepath.Join(src, "src", "remmina.c"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(c), "remmina_runtime_iconsdir()") {
		t.Fatal("upstream remmina.c not rewritten")
	}
}
