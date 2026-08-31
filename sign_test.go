package main

import "testing"

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
