package main

import (
	"context"
	"embed"
	"encoding/binary"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/actions-precompiled/foundation"
)

//go:embed launcher.c entitlements.plist
var bundleFS embed.FS

// Ad-hoc identity like rterm. install_name_tool invalidates Homebrew
// signatures; Apple Silicon then SIGKILLs on the first mapped page
// (CODESIGNING / Invalid Page) unless every Mach-O is re-signed.
func signApp(ctx context.Context, deps foundation.Deps, appDir string) error {
	if _, err := deps.Runner.Output(ctx, "which", "codesign"); err != nil {
		return fmt.Errorf("codesign: %w", ErrHostToolMissing)
	}
	var files []string
	err := filepath.WalkDir(appDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		if d.IsDir() || d.Type()&os.ModeSymlink != 0 {
			return nil
		}
		ok, err := fileIsMachO(path)
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		if ok {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("walk mach-o: %w", err)
	}
	if len(files) == 0 {
		return fmt.Errorf("sign: no mach-o files in %s", appDir)
	}
	ent := filepath.Join(appDir, "Contents", "entitlements.plist")
	data, err := bundleFS.ReadFile("entitlements.plist")
	if err != nil {
		return fmt.Errorf("embed entitlements: %w", err)
	}
	if err := deps.FS.WriteFile(ent, data, 0o644); err != nil {
		return err
	}
	defer func() { _ = deps.FS.RemoveAll(ent) }()

	// After exec the real remmina binary is the process; it needs the
	// disable-library-validation entitlement, not just the wrapper.
	privileged := map[string]bool{
		filepath.Join(appDir, "Contents", "MacOS", "Remmina"):            true,
		filepath.Join(appDir, "Contents", "Resources", "bin", "remmina"): true,
	}
	for _, f := range files {
		_ = deps.Runner.Run(ctx, "codesign", "--remove-signature", f)
		args := []string{"--force", "--sign", "-"}
		if privileged[f] {
			args = append(args, "--options", "runtime", "--entitlements", ent)
		}
		args = append(args, f)
		deps.Logf("codesign %s", f)
		if err := deps.Runner.Run(ctx, "codesign", args...); err != nil {
			return fmt.Errorf("codesign %s: %w", filepath.Base(f), err)
		}
	}
	deps.Logf("codesign app (ad-hoc, disable-library-validation)")
	if err := deps.Runner.Run(ctx, "codesign",
		"--force", "--sign", "-",
		"--options", "runtime",
		"--entitlements", ent,
		appDir,
	); err != nil {
		return fmt.Errorf("codesign app: %w", err)
	}
	return nil
}

func fileIsMachO(path string) (bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer f.Close()
	var magic uint32
	if err := binary.Read(f, binary.LittleEndian, &magic); err != nil {
		return false, nil
	}
	return isMachOMagic(magic), nil
}

func isMachOMagic(magic uint32) bool {
	switch magic {
	case 0xfeedface, 0xcefaedfe, 0xfeedfacf, 0xcffaedfe, 0xcafebabe, 0xbebafeca:
		return true
	default:
		return false
	}
}

func compileLauncher(ctx context.Context, deps foundation.Deps, dest string) error {
	src, err := bundleFS.ReadFile("launcher.c")
	if err != nil {
		return fmt.Errorf("embed launcher.c: %w", err)
	}
	dir, err := deps.FS.TempDir("", "remmina-launcher-")
	if err != nil {
		return err
	}
	defer deps.RemoveAllLog(dir, "launcher src")
	cfile := filepath.Join(dir, "launcher.c")
	if err := deps.FS.WriteFile(cfile, src, 0o644); err != nil {
		return err
	}
	if err := deps.Runner.Run(ctx, "cc", "-O2", "-Wl,-dead_strip", "-o", dest, cfile); err != nil {
		return fmt.Errorf("cc launcher: %w", err)
	}
	return os.Chmod(dest, 0o755)
}
