package main

import "path/filepath"

func remminaCMakeArgs(src, build, prefix, brew string) []string {
	inc := filepath.Join(brew, "include")
	return []string{
		"-G", "Ninja",
		"-S", src,
		"-B", build,
		"-DCMAKE_BUILD_TYPE=Release",
		"-DCMAKE_INSTALL_PREFIX=" + prefix,
		"-DCMAKE_PREFIX_PATH=" + brew,
		"-DCMAKE_C_FLAGS=-I" + inc,
		"-DCMAKE_OSX_DEPLOYMENT_TARGET=12.0",
		"-DWITH_FREERDP3=ON",
		"-DHAVE_LIBAPPINDICATOR=OFF",
		"-DWITH_CUPS=OFF",
		"-DWITH_ICON_CACHE=OFF",
		"-DWITH_UPDATE_DESKTOP_DB=OFF",
		"-DWITH_AVAHI=OFF",
		"-DWITH_TELEPATHY=OFF",
		"-DWITH_LIBSECRET=OFF",
		"-DWITH_WEBKIT2GTK=OFF",
		"-DWITH_WWW=OFF",
		"-DWITH_VTE=OFF",
		"-DWITH_PYTHONLIBS=OFF",
		"-DWITH_X2GO=OFF",
		"-DWITH_GVNC=OFF",
		"-DWITH_KF5WALLET=OFF",
		"-DWITH_NEWS=OFF",
		"-DWITH_STATS=OFF",
		"-DWITH_TIP=OFF",
		"-DWITH_MANPAGES=OFF",
	}
}

var requiredPlugins = []string{
	"remmina-plugin-rdp.so",
	"remmina-plugin-vnc.so",
	"remmina-plugin-exec.so",
}
