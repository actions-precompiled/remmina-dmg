#include "config.h"
#include "remmina_macos.h"

#ifdef __APPLE__

#include <stdlib.h>
#include <CoreFoundation/CoreFoundation.h>

static char macos_observer;

static gboolean
macos_prefers_dark(void)
{
	CFPropertyListRef appearance;
	gboolean dark = FALSE;

	CFPreferencesSynchronize(kCFPreferencesAnyApplication,
				 kCFPreferencesCurrentUser,
				 kCFPreferencesAnyHost);
	appearance = CFPreferencesCopyValue(CFSTR("AppleInterfaceStyle"),
					    kCFPreferencesAnyApplication,
					    kCFPreferencesCurrentUser,
					    kCFPreferencesAnyHost);
	if (appearance != NULL) {
		if (CFGetTypeID(appearance) == CFStringGetTypeID() &&
		    CFStringCompare((CFStringRef)appearance, CFSTR("Dark"), 0) == kCFCompareEqualTo) {
			dark = TRUE;
		}
		CFRelease(appearance);
	}
	return dark;
}

static gboolean
macos_apply_appearance(gpointer unused)
{
	GtkSettings *settings;
	GdkScreen *screen;
	gboolean dark;

	(void)unused;
	dark = macos_prefers_dark();
	/*
	 * GTK3 only loads gtk-dark.css from GTK_THEME=name:dark.
	 * prefer-dark is ignored while GTK_THEME is set, and Remmina keeps
	 * writing remmina_pref.dark_theme (off by default) over that property.
	 */
	setenv("GTK_THEME", dark ? "Adwaita:dark" : "Adwaita", 1);
	settings = gtk_settings_get_default();
	if (settings != NULL) {
		g_object_set(settings,
			     "gtk-application-prefer-dark-theme", dark,
			     NULL);
		g_object_notify(G_OBJECT(settings), "gtk-theme-name");
	}
	screen = gdk_screen_get_default();
	if (screen != NULL) {
		gtk_style_context_reset_widgets(screen);
	}
	return G_SOURCE_REMOVE;
}

static void
macos_appearance_changed(CFNotificationCenterRef center, void *observer,
			 CFStringRef name, const void *object,
			 CFDictionaryRef info)
{
	(void)center;
	(void)observer;
	(void)name;
	(void)object;
	(void)info;
	/* The notification can arrive before AppleInterfaceStyle is visible. */
	g_idle_add(macos_apply_appearance, NULL);
	g_timeout_add(150, macos_apply_appearance, NULL);
}

void
remmina_macos_init(void)
{
	GtkCssProvider *provider;

	provider = gtk_css_provider_new();
	/* Keep the floating fullscreen toolbar below the macOS menu bar. */
	gtk_css_provider_load_from_data(provider,
					"#remmina-connection-window-fullscreen {\n"
					"  padding-top: 28px;\n"
					"}\n"
					"#ftbbox-upper {\n"
					"  margin-top: 28px;\n"
					"}\n",
					-1, NULL);
	gtk_style_context_add_provider_for_screen(gdk_screen_get_default(),
						  GTK_STYLE_PROVIDER(provider),
						  GTK_STYLE_PROVIDER_PRIORITY_APPLICATION);
	g_object_unref(provider);

	macos_apply_appearance(NULL);
	CFNotificationCenterAddObserver(CFNotificationCenterGetDistributedCenter(),
					&macos_observer, macos_appearance_changed,
					CFSTR("AppleInterfaceThemeChangedNotification"),
					NULL, CFNotificationSuspensionBehaviorDeliverImmediately);
}

void
remmina_macos_adapt_main_window(GtkWindow *win)
{
	GtkWidget *hb, *child;

	if (win == NULL) {
		return;
	}
	/* remmina_main_new writes remmina_pref.dark_theme over GtkSettings. */
	macos_apply_appearance(NULL);
	hb = gtk_window_get_titlebar(win);
	if (hb == NULL) {
		return;
	}
	g_object_ref(hb);
	gtk_window_set_titlebar(win, NULL);
	if (GTK_IS_HEADER_BAR(hb)) {
		gtk_header_bar_set_show_close_button(GTK_HEADER_BAR(hb), FALSE);
	}
	child = gtk_bin_get_child(GTK_BIN(win));
	if (GTK_IS_BOX(child)) {
		gtk_box_pack_start(GTK_BOX(child), hb, FALSE, FALSE, 0);
		gtk_box_reorder_child(GTK_BOX(child), hb, 0);
		gtk_widget_show(hb);
	} else {
		gtk_window_set_titlebar(win, hb);
	}
	g_object_unref(hb);
}

#else

void
remmina_macos_init(void)
{
}

void
remmina_macos_adapt_main_window(GtkWindow *win)
{
	(void)win;
}

#endif
