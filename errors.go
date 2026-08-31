package main

import "errors"

var (
	ErrNeedDarwin        = errors.New("remmina DMGs must be built on macOS")
	ErrUnsupportedTarget = errors.New("unsupported target")
	ErrHostMismatch      = errors.New("target does not match this Mac")
	ErrCloneFailed       = errors.New("clone upstream failed")
	ErrHostToolMissing   = errors.New("required host tool missing")
	ErrBrewPrefix        = errors.New("brew --prefix failed")
	ErrRemminaMissing    = errors.New("remmina binary missing after install")
	ErrPluginMissing     = errors.New("expected remmina plugin missing")
	ErrInject            = errors.New("bundle path inject failed")
	ErrDylibbundler      = errors.New("dylibbundler failed")
	ErrDMG               = errors.New("hdiutil dmg failed")
	ErrSmokeNoArtifacts  = errors.New("smoke: no artifacts")
	ErrSmokeAppMissing   = errors.New("smoke: Remmina.app missing in DMG")
	ErrHomebrewLink      = errors.New("smoke: binary still linked to Homebrew")
	ErrAppIconMissing    = errors.New("app icon source png missing")
	ErrAppIconICNS       = errors.New("AppIcon.icns missing")
	ErrBadInstallName    = errors.New("bundled dylib has a non-sibling install name")
)
