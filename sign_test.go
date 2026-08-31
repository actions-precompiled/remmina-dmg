package main

import (
	"os"
	"strings"
	"testing"
)

func TestLauncherLeavesAdwaitaUnlocked(t *testing.T) {
	t.Parallel()
	data, err := os.ReadFile("launcher.c")
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	if strings.Contains(s, "Adwaita:dark") {
		t.Fatal("GTK_THEME must not lock Adwaita:dark")
	}
	if !strings.Contains(s, `unsetenv("GTK_THEME")`) {
		t.Fatal("expected unsetenv GTK_THEME so prefer-dark is consulted")
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
