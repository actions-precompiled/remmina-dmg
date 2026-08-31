# actions-precompiled / remmina

Unsigned **[Remmina](https://remmina.org/)** `.app` inside a DMG, built with
[`foundation`](https://github.com/actions-precompiled/foundation).

This repo ships **DMGs only**, for **Apple Silicon**. Intel is the same Work
path (`--targets darwin-amd64` on an Intel Mac) but is not in CI: Homebrew
gtk+3 / freerdp bottles stop at Sonoma on Intel, and GitHub’s last Intel
runner is Ventura.

The DMG is ad-hoc signed (`codesign --sign -`), same idea as rterm. Gatekeeper
will warn. After dragging to Applications:

```bash
xattr -cr /Applications/Remmina.app
```

## What is in the app

RDP (FreeRDP 3), VNC, SSH, EXEC. GTK3 + those libs are copied into the bundle
with `dylibbundler`. No SPICE, WebKit, VTE, Python, Avahi, or secret plugin.

Upstream is GitLab (`v*` tags). The GitHub Remmina mirror is not used for
versions — its releases are years behind.

## CLI

```bash
mise install
mise exec -- go run . plan
mise exec -- go run . list
mise exec -- go run . list --all
mise exec -- go run . build v1.4.43          # macOS only
mise exec -- go run . generate workflow --force
```

## Layout

```text
remmina-1.4.43-darwin-aarch64.dmg
└── Remmina.app
    └── Contents/
        ├── MacOS/Remmina          # launcher
        └── Resources/
            ├── bin/remmina
            ├── lib/               # remmina plugins + vendored dylibs
            └── share/
```

## CI

- `build.yml` — `macos-latest`, one tag per run
- `dispatch-missing.yml` — `go run . list` then one Build per tag

## License

MIT for packaging. Remmina is GPLv2+ (see upstream).
