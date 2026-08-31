#include "config.h"
#include "remmina_bundle.h"

#ifdef __APPLE__
#include <mach-o/dyld.h>
#include <limits.h>
#include <stdlib.h>
#endif

static const gchar *
bundle_prefix(void)
{
#ifdef __APPLE__
	static gchar *prefix;
	static gboolean checked;
	if (checked) {
		return prefix;
	}
	checked = TRUE;

	char exe[PATH_MAX];
	uint32_t n = sizeof(exe);
	if (_NSGetExecutablePath(exe, &n) != 0) {
		return NULL;
	}
	char real[PATH_MAX];
	if (!realpath(exe, real)) {
		return NULL;
	}
	gchar *dir = g_path_get_dirname(real);
	gchar *cand = g_path_get_dirname(dir);
	g_free(dir);
	gchar *marker = g_build_filename(cand, "share", "remmina", NULL);
	if (g_file_test(marker, G_FILE_TEST_IS_DIR)) {
		prefix = cand;
		g_free(marker);
		return prefix;
	}
	g_free(marker);
	g_free(cand);
#endif
	return NULL;
}

static const gchar *
join_or(const gchar *fallback, const gchar *a, const gchar *b)
{
	const gchar *p = bundle_prefix();
	if (p == NULL) {
		return fallback;
	}
	if (b == NULL) {
		return g_build_filename(p, a, NULL);
	}
	return g_build_filename(p, a, b, NULL);
}

const gchar *
remmina_runtime_datadir(void)
{
	static const gchar *v;
	if (v == NULL) {
		v = join_or(REMMINA_RUNTIME_DATADIR, "share", NULL);
	}
	return v;
}

const gchar *
remmina_runtime_iconsdir(void)
{
	static const gchar *v;
	if (v == NULL) {
		v = join_or(NULL, "share", "icons");
		if (v == NULL) {
			v = REMMINA_RUNTIME_DATADIR G_DIR_SEPARATOR_S "icons";
		}
	}
	return v;
}

const gchar *
remmina_runtime_localedir(void)
{
	static const gchar *v;
	if (v == NULL) {
		v = join_or(REMMINA_RUNTIME_LOCALEDIR, "share", "locale");
	}
	return v;
}

const gchar *
remmina_runtime_plugindir(void)
{
	static const gchar *v;
	if (v == NULL) {
		v = join_or(REMMINA_RUNTIME_PLUGINDIR, "lib", "remmina/plugins");
	}
	return v;
}

const gchar *
remmina_runtime_uidir(void)
{
	static const gchar *v;
	if (v == NULL) {
		v = join_or(REMMINA_RUNTIME_UIDIR, "share", "remmina/ui");
	}
	return v;
}

const gchar *
remmina_runtime_external_tools_dir(void)
{
	static const gchar *v;
	if (v == NULL) {
		v = join_or(REMMINA_RUNTIME_EXTERNAL_TOOLS_DIR, "share", "remmina/external_tools");
	}
	return v;
}

const gchar *
remmina_runtime_term_cs_dir(void)
{
	static const gchar *v;
	if (v == NULL) {
		v = join_or(REMMINA_RUNTIME_TERM_CS_DIR, "share", "remmina/theme");
	}
	return v;
}
