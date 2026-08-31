package main

import (
	"os"
	"strings"
	"testing"
)

func TestAdwaitaVariantComesFromGTKTheme(t *testing.T) {
	t.Parallel()
	launch, err := os.ReadFile("launcher.c")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(launch), `setenv("GTK_THEME", dark ? "Adwaita:dark" : "Adwaita", 1)`) {
		t.Fatal("expected launcher to set GTK_THEME from appearance")
	}
	mac, err := os.ReadFile("inject/remmina_macos.c")
	if err != nil {
		t.Fatal(err)
	}
	s := string(mac)
	if !strings.Contains(s, `setenv("GTK_THEME", dark ? "Adwaita:dark" : "Adwaita", 1)`) {
		t.Fatal("expected remmina_macos to update GTK_THEME at runtime")
	}
	if strings.Contains(s, `unsetenv("GTK_THEME")`) {
		t.Fatal("unsetenv GTK_THEME drops the dark variant")
	}
}

func TestIsMachOMagic(t *testing.T) {
	t.Parallel()
	if !isMachOMagic(0xfeedfacf) || !isMachOMagic(0xcffaedfe) {
		t.Fatal("arm64/little-endian 64-bit")
	}
	if !isMachOMagic(0xcafebabe) {
		t.Fatal("fat")
	}
	if isMachOMagic(0x7f454c46) { // ELF
		t.Fatal("elf is not macho")
	}
	if isMachOMagic(0x23212f62) { // #!/b
		t.Fatal("script is not macho")
	}
}
