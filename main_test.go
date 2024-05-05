package main

import (
	"bufio"
	"bytes"
	"testing"
)

const stanza1 = `Package: test-pkg1
Version: 1.0.0
Section: 
Priority: optional
Architecture: all
Maintainer: Unset Maintainer <unset@localhost>
Installed-Size: 0
Description: no description given
Filename: ./examples/test-pkg1_1.0.0_all.deb
Size: 510
MD5sum: 0df74607c7ce1414c7b5d4a1a9f01e80
SHA1: 5a3f1d87ea4db2c204803ceb9400c828aae211d9
SHA256: 9b6e51e7d30e5a8b9bebaeeb7383910c31f82d655e68d45c65d9dc647b920d07

`

const stanza2 = `Package: test-pkg2
Version: 1.0.0
Section: 
Priority: optional
Architecture: amd64
Maintainer: Unset Maintainer <unset@localhost>
Installed-Size: 0
Description: no description given
Filename: ./examples/test-pkg2_1.0.0_amd64.deb
Size: 512
MD5sum: aeda61515dbdf90cd08922254e33abcc
SHA1: 5cf1ef88800084ac70faf69f27ec807782caf092
SHA256: a0af1aa05a98536b21e1a60ae5199d0d749b0e9d81fca12aeb13d10c1e53b055

`

func TestExtractStanza(t *testing.T) {
	var buf bytes.Buffer
	w := bufio.NewWriter(&buf)
	err := ExtractStanza("./examples/test-pkg1_1.0.0_all.deb", "", w)
	if err != nil {
		t.Error(err)
	}
	w.Flush()
	got := buf.String()
	if got != stanza1 {
		t.Errorf("Expected:\n%s\ngot:\n%s", stanza1, got)
	}
}

func TestScanPackages(t *testing.T) {
	var buf bytes.Buffer
	w := bufio.NewWriter(&buf)
	i, err := ScanPackages(".", 2, w)
	if err != nil {
		t.Error(err)
	}
	if i != 2 {
		t.Errorf("Expected 2 packages, got %d", i)
	}
	w.Flush()
	got := buf.String()
	expected := stanza1 + stanza2
	if got != expected {
		t.Errorf("Expected:\n%s\ngot:\n%s", expected, got)
	}
}
