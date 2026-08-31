package main

import (
	"embed"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/actions-precompiled/foundation"
)

//go:embed inject/remmina_bundle.c inject/remmina_bundle.h inject/remmina_macos.c inject/remmina_macos.h
var injectFS embed.FS

const injectMarker = ".apc-bundle-inject"

func injectBundleSources(deps foundation.Deps, src string) error {
	marker := filepath.Join(src, injectMarker)
	if _, err := deps.FS.Stat(marker); err == nil {
		deps.Logf("inject: already applied in %s", src)
		return nil
	}

	srcDir := filepath.Join(src, "src")
	for _, name := range []string{"remmina_bundle.c", "remmina_bundle.h", "remmina_macos.c", "remmina_macos.h"} {
		data, err := injectFS.ReadFile("inject/" + name)
		if err != nil {
			return fmt.Errorf("%w: embed %s: %w", ErrInject, name, err)
		}
		if err := deps.FS.WriteFile(filepath.Join(srcDir, name), data, 0o644); err != nil {
			return fmt.Errorf("%w: write %s: %w", ErrInject, name, err)
		}
	}

	rootCMake := filepath.Join(src, "CMakeLists.txt")
	if err := replaceOnce(deps, rootCMake,
		`if(NOT FREEBSD)
  set(CMAKE_INSTALL_RPATH "\$ORIGIN/../${CMAKE_INSTALL_LIBDIR}:\$ORIGIN/..")
endif()`,
		`if(APPLE)
  set(CMAKE_INSTALL_RPATH "@loader_path/../lib")
  set(CMAKE_BUILD_RPATH_USE_ORIGIN FALSE)
elseif(NOT FREEBSD)
  set(CMAKE_INSTALL_RPATH "\$ORIGIN/../${CMAKE_INSTALL_LIBDIR}:\$ORIGIN/..")
endif()`,
	); err != nil {
		return err
	}

	cmake := filepath.Join(srcDir, "CMakeLists.txt")
	if err := replaceOnce(deps, cmake,
		"  \"remmina_utils.h\"\n",
		"  \"remmina_utils.h\"\n  \"remmina_bundle.c\"\n  \"remmina_bundle.h\"\n  \"remmina_macos.c\"\n  \"remmina_macos.h\"\n",
	); err != nil {
		return err
	}

	replacements := []struct {
		file, old, neu string
	}{
		{
			filepath.Join(srcDir, "remmina.c"),
			"#include \"remmina_public.h\"\n",
			"#include \"remmina_public.h\"\n#include \"remmina_bundle.h\"\n#include \"remmina_macos.h\"\n",
		},
		{
			filepath.Join(srcDir, "remmina.c"),
			"REMMINA_RUNTIME_DATADIR G_DIR_SEPARATOR_S \"icons\");",
			"remmina_runtime_iconsdir());\n	remmina_macos_init();",
		},
		{
			filepath.Join(srcDir, "remmina.c"),
			"bindtextdomain(GETTEXT_PACKAGE, REMMINA_RUNTIME_LOCALEDIR);",
			"bindtextdomain(GETTEXT_PACKAGE, remmina_runtime_localedir());",
		},
		{
			filepath.Join(srcDir, "remmina_plugin_manager.c"),
			"#include \"remmina_utils.h\"\n",
			"#include \"remmina_utils.h\"\n#include \"remmina_bundle.h\"\n",
		},
		{
			filepath.Join(srcDir, "remmina_plugin_manager.c"),
			"g_ptr_array_add(plugin_dirs, REMMINA_RUNTIME_PLUGINDIR);",
			"g_ptr_array_add(plugin_dirs, (gpointer) remmina_runtime_plugindir());",
		},
		{
			filepath.Join(srcDir, "remmina_public.c"),
			"#include \"config.h\"\n",
			"#include \"config.h\"\n#include \"remmina_bundle.h\"\n",
		},
		{
			filepath.Join(srcDir, "remmina_public.c"),
			"gchar *ui_path = g_strconcat(REMMINA_RUNTIME_UIDIR, G_DIR_SEPARATOR_S, filename, NULL);",
			"gchar *ui_path = g_build_filename(remmina_runtime_uidir(), filename, NULL);",
		},
		{
			filepath.Join(srcDir, "remmina_external_tools.c"),
			"#include \"remmina_external_tools.h\"\n",
			"#include \"remmina_external_tools.h\"\n#include \"remmina_bundle.h\"\n",
		},
		{
			filepath.Join(srcDir, "remmina_external_tools.c"),
			"strcpy(dirname, REMMINA_RUNTIME_EXTERNAL_TOOLS_DIR);",
			"g_strlcpy(dirname, remmina_runtime_external_tools_dir(), MAX_PATH_LEN);",
		},
		{
			filepath.Join(srcDir, "remmina_external_tools.c"),
			"g_snprintf(launcher, MAX_PATH_LEN, \"%s/launcher.sh\", REMMINA_RUNTIME_EXTERNAL_TOOLS_DIR);",
			"g_snprintf(launcher, MAX_PATH_LEN, \"%s/launcher.sh\", remmina_runtime_external_tools_dir());",
		},
		{
			filepath.Join(srcDir, "remmina_pref_dialog.c"),
			"#include \"remmina_public.h\"\n",
			"#include \"remmina_public.h\"\n#include \"remmina_bundle.h\"\n",
		},
		{
			filepath.Join(srcDir, "remmina_pref_dialog.c"),
			"gtk_file_chooser_set_current_folder(GTK_FILE_CHOOSER(remmina_pref_dialog->button_term_cs), REMMINA_RUNTIME_TERM_CS_DIR);",
			"gtk_file_chooser_set_current_folder(GTK_FILE_CHOOSER(remmina_pref_dialog->button_term_cs), remmina_runtime_term_cs_dir());",
		},
		{
			filepath.Join(srcDir, "remmina_main.c"),
			"#include \"remmina_unlock.h\"\n",
			"#include \"remmina_unlock.h\"\n#include \"remmina_macos.h\"\n",
		},
		{
			filepath.Join(srcDir, "remmina_main.c"),
			"remminamain->builder = remmina_public_gtk_builder_new_from_resource(\"/org/remmina/Remmina/src/../data/ui/remmina_main.glade\");\n	remminamain->window = GTK_WINDOW(RM_GET_OBJECT(\"RemminaMain\"));",
			"remminamain->builder = remmina_public_gtk_builder_new_from_resource(\"/org/remmina/Remmina/src/../data/ui/remmina_main.glade\");\n	remminamain->window = GTK_WINDOW(RM_GET_OBJECT(\"RemminaMain\"));\n	remmina_macos_adapt_main_window(remminamain->window);",
		},
		{
			filepath.Join(srcDir, "remmina_pref.c"),
			"	if (!keymap || keymap[0] == '\\0')\n		return keyval;",
			"	if (!keymap || keymap[0] == '\\0') {\n#ifdef __APPLE__\n		if (keyval == GDK_KEY_Meta_L)\n			return GDK_KEY_Super_L;\n		if (keyval == GDK_KEY_Meta_R)\n			return GDK_KEY_Super_R;\n#endif\n		return keyval;\n	}",
		},
	}
	for _, r := range replacements {
		if err := replaceOnce(deps, r.file, r.old, r.neu); err != nil {
			return err
		}
	}
	return deps.FS.WriteFile(marker, []byte("apple-bundle-paths\n"), 0o644)
}

func replaceOnce(deps foundation.Deps, path, old, neu string) error {
	data, err := deps.FS.ReadFile(path)
	if err != nil {
		return fmt.Errorf("%w: read %s: %w", ErrInject, path, err)
	}
	s := string(data)
	if !strings.Contains(s, old) {
		return fmt.Errorf("%w: %s: pattern not found", ErrInject, filepath.Base(path))
	}
	if strings.Count(s, old) != 1 {
		return fmt.Errorf("%w: %s: pattern matched %d times", ErrInject, filepath.Base(path), strings.Count(s, old))
	}
	return deps.FS.WriteFile(path, []byte(strings.Replace(s, old, neu, 1)), 0o644)
}
