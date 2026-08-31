//go:build ignore

#include <limits.h>
#include <mach-o/dyld.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>

static int
join(char *out, size_t n, const char *a, const char *b)
{
	int w = snprintf(out, n, "%s/%s", a, b);
	return w < 0 || (size_t)w >= n;
}

static void
set_pref(const char *key, const char *dir, const char *rel)
{
	char val[PATH_MAX];
	if (join(val, sizeof(val), dir, rel)) {
		return;
	}
	setenv(key, val, 1);
}

static void
write_pixbuf_cache(const char *res)
{
	char src[PATH_MAX], loaders[PATH_MAX], dst[PATH_MAX];
	if (join(src, sizeof(src), res, "lib/gdk-pixbuf-2.0/2.10.0/loaders.cache")) {
		return;
	}
	if (join(loaders, sizeof(loaders), res, "lib/gdk-pixbuf-2.0/2.10.0/loaders")) {
		return;
	}
	const char *tmp = getenv("TMPDIR");
	if (tmp == NULL || tmp[0] == '\0') {
		tmp = "/tmp";
	}
	if (snprintf(dst, sizeof(dst), "%s/remmina-pixbuf-loaders.cache", tmp) < 0) {
		return;
	}
	FILE *in = fopen(src, "r");
	if (in == NULL) {
		return;
	}
	FILE *out = fopen(dst, "w");
	if (out == NULL) {
		fclose(in);
		return;
	}
	char line[4096];
	const char *tok = "@REMMINA_LOADERS@";
	size_t toklen = strlen(tok);
	while (fgets(line, sizeof(line), in) != NULL) {
		char *hit = strstr(line, tok);
		if (hit == NULL) {
			fputs(line, out);
			continue;
		}
		*hit = '\0';
		fputs(line, out);
		fputs(loaders, out);
		fputs(hit + toklen, out);
	}
	fclose(in);
	fclose(out);
	setenv("GDK_PIXBUF_MODULE_FILE", dst, 1);
}

static void
prepend_path(const char *key, const char *dir, const char *rel)
{
	char val[PATH_MAX];
	if (join(val, sizeof(val), dir, rel)) {
		return;
	}
	const char *old = getenv(key);
	if (old == NULL || old[0] == '\0') {
		setenv(key, val, 1);
		return;
	}
	char both[PATH_MAX * 2];
	if (snprintf(both, sizeof(both), "%s:%s", val, old) < 0) {
		return;
	}
	setenv(key, both, 1);
}

int
main(int argc, char **argv)
{
	char exe[PATH_MAX];
	uint32_t n = sizeof(exe);
	if (_NSGetExecutablePath(exe, &n) != 0) {
		return 127;
	}
	char real[PATH_MAX];
	if (realpath(exe, real) == NULL) {
		return 127;
	}
	char *slash = strrchr(real, '/');
	if (slash == NULL) {
		return 127;
	}
	*slash = '\0';

	char res[PATH_MAX];
	if (join(res, sizeof(res), real, "../Resources")) {
		return 127;
	}
	char resreal[PATH_MAX];
	if (realpath(res, resreal) == NULL) {
		return 127;
	}

	prepend_path("XDG_DATA_DIRS", resreal, "share");
	prepend_path("XDG_CONFIG_DIRS", resreal, "etc");
	set_pref("GSETTINGS_SCHEMA_DIR", resreal, "share/glib-2.0/schemas");
	set_pref("GDK_PIXBUF_MODULEDIR", resreal, "lib/gdk-pixbuf-2.0/2.10.0/loaders");
	write_pixbuf_cache(resreal);
	setenv("GTK_DATA_PREFIX", resreal, 1);
	setenv("GTK_EXE_PREFIX", resreal, 1);
	setenv("GTK_PATH", resreal, 1);
	prepend_path("GI_TYPELIB_PATH", resreal, "lib/girepository-1.0");
	set_pref("GIO_MODULE_DIR", resreal, "lib/gio/modules");
	set_pref("PANGO_LIBDIR", resreal, "lib");

	/* No :dark suffix — remmina_macos flips prefer-dark on Appearance changes. */
	setenv("GTK_THEME", "Adwaita", 1);

	char bin[PATH_MAX];
	if (join(bin, sizeof(bin), resreal, "bin/remmina")) {
		return 127;
	}
	execv(bin, argv);
	perror(bin);
	return 127;
}
