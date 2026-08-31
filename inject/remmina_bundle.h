#ifndef REMMINA_BUNDLE_H
#define REMMINA_BUNDLE_H

#include <glib.h>

const gchar *remmina_runtime_datadir(void);
const gchar *remmina_runtime_iconsdir(void);
const gchar *remmina_runtime_localedir(void);
const gchar *remmina_runtime_plugindir(void);
const gchar *remmina_runtime_uidir(void);
const gchar *remmina_runtime_external_tools_dir(void);
const gchar *remmina_runtime_term_cs_dir(void);

#endif
