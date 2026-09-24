package native

// Phase 147A (evidence): dumps a linked native image of the `empty`
// bisect program to a caller-chosen directory for out-of-band forensics
// (`readelf -h -l`, `strace -f`). Bytes only — runs on every platform;
// only runs at all when KARKAIN_NATIVE_DUMP=<dir> is set, so default CI
// and every mandatory gate are untouched. Informational, never red:
// a dump failure fails this test alone (a diagnostics-only test), and
// the CI job that sets the variable is continue-on-error.

import (
	"os"
	"path/filepath"
	"testing"

	"karkain/pkg/lexer"
	"karkain/pkg/parser"
)

func TestNativeEvidenceDump(t *testing.T) {
	dir := os.Getenv("KARKAIN_NATIVE_DUMP")
	if dir == "" {
		t.Skip("evidence dump off (set KARKAIN_NATIVE_DUMP=<dir>)")
	}
	l := lexer.New("func main() {\n}\n")
	p := parser.New(l)
	prog := p.ParseProgram()
	if len(p.Errors) > 0 {
		t.Fatalf("parse: %v", p.Errors)
	}
	img, err := CompileProgram(prog)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if _, _, err := Parse(img); err != nil {
		t.Fatalf("structural parse of linked image: %v", err)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	out := filepath.Join(dir, "native-empty")
	if err := os.WriteFile(out, img, 0o755); err != nil {
		t.Fatalf("write: %v", err)
	}
	t.Logf("dumped %d-byte image to %s", len(img), out)
}
