package cli

import (
	"path/filepath"
	"testing"
)

// Karkain source discovery accepts .kark and does not treat .kar as the
// canonical Karkain source extension.
func TestKarkainSourceExtensionIsKark(t *testing.T) {
	root := t.TempDir()

	canonical := filepath.Join(root, "main.kark")
	legacy := filepath.Join(root, "legacy.kar")
	canonicalTest := filepath.Join(root, "math_test.kark")
	legacyTest := filepath.Join(root, "old_test.kar")

	writeFile(t, canonical, "func main() { print(1) }\n")
	writeFile(t, legacy, "func main() { print(1) }\n")
	writeFile(t, canonicalTest, "func test_x() { assert(1 == 1) }\n")
	writeFile(t, legacyTest, "func test_x() { assert(1 == 1) }\n")

	// A .kark source is accepted by the source-file gate.
	if err := ValidateKarFile(canonical); err != nil {
		t.Fatalf("ValidateKarFile rejected canonical .kark source: %v", err)
	}

	// A .kar file must not be accepted as Karkain source.
	if err := ValidateKarFile(legacy); err == nil {
		t.Fatal("ValidateKarFile accepted .kar as a Karkain source extension")
	}

	// Test discovery picks up *_test.kark and ignores *_test.kar.
	files, err := findTestFiles(root)
	if err != nil {
		t.Fatalf("findTestFiles: %v", err)
	}
	if len(files) != 1 || files[0] != canonicalTest {
		t.Fatalf("findTestFiles = %v; want exactly [%s] (.kar must not be canonical)", files, canonicalTest)
	}
}