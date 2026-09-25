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

	// Phase 147C: the `call` bisect program exercises multi-arg calls
	// plus print (the second-defect surface) — dump it alongside.
	l2 := lexer.New("func add(a, b) {\n    return a + b\n}\nfunc main() {\n    print(add(20, 22))\n}\n")
	p2 := parser.New(l2)
	prog2 := p2.ParseProgram()
	if len(p2.Errors) > 0 {
		t.Fatalf("parse call: %v", p2.Errors)
	}
	img2, err := CompileProgram(prog2)
	if err != nil {
		t.Fatalf("compile call: %v", err)
	}
	if _, _, err := Parse(img2); err != nil {
		t.Fatalf("structural parse of call image: %v", err)
	}
	out2 := filepath.Join(dir, "native-call")
	if err := os.WriteFile(out2, img2, 0o755); err != nil {
		t.Fatalf("write call: %v", err)
	}
	t.Logf("dumped %d-byte image to %s", len(img2), out2)

	// Phase 149: same programs for the PE and Mach-O containers, so
	// objdump/readelf-class forensics work for every format.
	dumpOS := func(osName, name, src string) {
		t.Helper()
		l := lexer.New(src)
		p := parser.New(l)
		prog := p.ParseProgram()
		if len(p.Errors) > 0 {
			t.Fatalf("parse %s: %v", name, p.Errors)
		}
		img, err := CompileProgramForOS(prog, osName)
		if err != nil {
			t.Fatalf("compile %s: %v", name, err)
		}
		out := filepath.Join(dir, name)
		if err := os.WriteFile(out, img, 0o755); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
		t.Logf("dumped %d-byte image to %s", len(img), out)
	}
	dumpOS(OSWindows, "native-empty-pe.exe", "func main() {\n}\n")
	dumpOS(OSMacOS, "native-empty-macho", "func main() {\n}\n")
	dumpOS(OSWindows, "native-retcode-pe.exe", "func main() {\n    return 7\n}\n")
	dumpOS(OSWindows, "native-int42-pe.exe", "func main() {\n    print(42)\n}\n")
}
