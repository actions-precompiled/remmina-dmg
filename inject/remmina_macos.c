#include "config.h"
#include "remmina_macos.h"

#ifdef __APPLE__

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
}

void
remmina_macos_adapt_main_window(GtkWindow *win)
{
	GtkWidget *hb, *child;

	if (win == NULL) {
		return;
	}
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
