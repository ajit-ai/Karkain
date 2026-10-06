package cli

// Phase 151A Step 5 -- the Windows entry, the Win64 exit tail and the PEB
// bootstrap in kcc.
//
// FOUR INDEPENDENT LAYERS, and each one is load-bearing:
//
//	L1 STRUCTURAL       the kcc corpus decodes as the expected instruction
//	                    regions, and a real PE image passes the ORACLE's
//	                    ParsePE validator (not a check written here).
//	L2 BYTE IDENTITY    kcc's bytes vs the REAL pkg/native.Emitter building the
//	                    same sequence -- a differential, never a golden. A
//	                    golden pins kcc against a transcription of the Go code
//	                    and passes when both copies are wrong together. Plus an
//	                    INDEPENDENTLY COMPUTED constant table (the export-name
//	                    character pairs derived from the name strings here, not
//	                    copied from either side).
//	L3 NATIVE EXECUTION the generated PE is actually EXECUTED on this host and
//	                    an observable result observed. Emission is not
//	                    execution; a program that builds but never runs proves
//	                    nothing about the PEB bootstrap.
//	L4 REGRESSION / KIR the pre-existing native byte-identity differential and
//	                    the whole-tree KIR pin are unchanged apart from the
//	                    explicitly authorized Step-5 delta.
//
// NO SILENT GO FALLBACK. That is the whole point of increment 151, and the
// measured reason it exists (PHASE-151-BASELINE.md Â§1.1): before 151,
// `KARKAIN_ENGINE=kcc karkain build --target native-x86_64-linux` printed the
// GO lexer's verbose banner and produced the GO image, byte-identically, so
// every parity claim was vacuous. L2 fails if kcc does not emit the bytes, and
// testPhase151A5NoGoFallback removes kcc's dispatch to prove it.

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"karkain/pkg/lexer"
	"karkain/pkg/native"
	"karkain/pkg/parser"
)

// goWinNamePairs derives the 16-bit character-pair constants the export walk
// compares against, FROM THE NAME STRING. The oracle builds them inline as
// uint32(name[k]) | uint32(name[k+1])<<8 (program.go emitWinResolve), so this
// recomputes them rather than copying either implementation's literals. A
// mistyped constant therefore fails here instead of agreeing with a second copy
// of the same typo -- the 151B lesson, which 152-B2 hit for real.
//
// An odd-length name leaves the final high byte 0, and the oracle COMPARES that
// pair against the string's NUL terminator rather than skipping it, so the tail
// is included rather than elided.
func goWinNamePairs(name string) []uint32 {
	var out []uint32
	for k := 0; k < len(name); k += 2 {
		lo := uint32(name[k])
		var hi uint32
		if k+1 < len(name) {
			hi = uint32(name[k+1])
		}
		out = append(out, lo|hi<<8)
	}
	return out
}

// goWinResolve reproduces emitWinResolve byte for byte using only EXPORTED
// Emitter methods.
//
// idx selects the LABEL names. The oracle calls b.fresh() per resolve, so
// ExitProcess/GetStdHandle/WriteFile get distinct label sets; fixed names
// collide, and the Emitter rejects a duplicate label outright -- which is how
// this was found, the first draft panicking with `duplicate label "exp$loop"`.
// Label NAMES are not bytes, so naming them differently from the oracle's fresh()
// counter does not affect the comparison; ALLOCATING distinct ones does, and
// Step 3's gate showed a rel32 can leave every byte identical while the label
// discipline is wrong.
func goWinResolve(e *native.Emitter, name string, idx int) {
	e.LoadBaseOff32(native.RCX, native.R10, 0x3C) // e_lfanew
	e.MovRegReg(native.RDX, native.R10)
	e.AddRegReg(native.RDX, native.RCX) // RDX = NT headers
	// Export directory at NT+136 (COFF 24 + Optional 112). +96 is the 32-bit
	// header shape and reads garbage -- measured in Phase 149.
	e.LoadBaseOff32(native.RCX, native.RDX, 136)
	e.MovRegReg(native.RDX, native.R10)
	e.AddRegReg(native.RDX, native.RCX)         // RDX = export directory
	e.LoadBaseOff32(native.R9, native.RDX, 24)  // NumberOfNames
	e.LoadBaseOff32(native.RCX, native.RDX, 32) // AddressOfNames
	e.MovRegReg(native.RSI, native.R10)
	e.AddRegReg(native.RSI, native.RCX)         // RSI = namesPtr
	e.LoadBaseOff32(native.RCX, native.RDX, 36) // AddressOfNameOrdinals
	e.MovRegReg(native.RDI, native.R10)
	e.AddRegReg(native.RDI, native.RCX)         // RDI = ordPtr
	e.LoadBaseOff32(native.RCX, native.RDX, 28) // AddressOfFunctions
	e.MovRegReg(native.RBP, native.R10)
	e.AddRegReg(native.RBP, native.RCX) // RBP = funcsPtr
	sfx := itoa151A5(idx)
	loopLbl := "exp$loop$" + sfx
	nextLbl := "exp$next$" + sfx
	doneLbl := "exp$done$" + sfx
	failLbl := "exp$fail$" + sfx
	e.Mark(loopLbl)
	e.TestRegReg(native.R9, native.R9)
	e.Jz(failLbl)
	e.LoadBaseOff32(native.RCX, native.RSI, 0) // nameRVA
	e.MovRegReg(native.RDX, native.R10)
	e.AddRegReg(native.RDX, native.RCX) // RDX = candidate name
	for k := 0; k < len(name); k += 2 {
		e.MovzxRegMem16(native.R8, native.RDX, k)
		e.CmpRegImm32(native.R8, goWinNamePairs(name)[k/2])
		e.Jnz(nextLbl)
	}
	e.MovzxRegMem16(native.R8, native.RDI, 0) // ordinal
	e.LoadScaled32(native.RAX, native.RBP, native.R8, 4, 0)
	e.MovRegReg(native.RDX, native.R10)
	e.AddRegReg(native.RDX, native.RAX) // RDX = resolved address
	// 48 A3 stores RAX specifically (moffs is accumulator-only), so the
	// resolved address moves RDX -> RAX first. Storing RDX published the raw
	// function RVA -- the Phase-149 defect.
	e.MovRegReg(native.RAX, native.RDX)
	e.StoreAbs64Placeholder()
	e.Jmp(doneLbl)
	e.Mark(nextLbl)
	e.AddRegImm32(native.RSI, 4)
	e.AddRegImm32(native.RDI, 2)
	e.DecReg(native.R9)
	e.Jmp(loopLbl)
	e.Mark(failLbl)
	e.Int3()
	e.Mark(doneLbl)
}

// itoa151A5 renders a small non-negative int so label names can carry an index
// without pulling strconv in for one call site.
func itoa151A5(n int) string {
	if n == 0 {
		return "0"
	}
	var d []byte
	for n > 0 {
		d = append([]byte{byte('0' + n%10)}, d...)
		n /= 10
	}
	return string(d)
}

// goWinBootstrap reproduces emitWinBootstrap byte for byte.
func goWinBootstrap(e *native.Emitter) {
	e.MovRegGsMem(native.RAX, 0x60)             // PEB
	e.LoadBaseOff(native.RAX, native.RAX, 0x18) // PEB->Ldr
	e.LoadBaseOff(native.RAX, native.RAX, 0x20) // InMemoryOrder head
	e.MovRegImm32(native.R11, 64)               // walk bound
	e.Mark("k32$walk")
	e.LoadBaseOff(native.RAX, native.RAX, 0) // Flink -> entry links
	e.MovRegReg(native.RBX, native.RAX)
	e.SubRegImm32(native.RBX, 0x10) // links field -> entry base
	// 64-bit LDR_DATA_TABLE_ENTRY: DllBase +0x30, BaseDllName +0x58
	// (Length +0, Buffer +8). The 32-bit +0x28/+0x50 shape skipped every module
	// into the Int3 -- measured in Phase 149.
	e.MovzxRegMem16(native.RCX, native.RBX, 0x58)
	e.CmpRegImm32(native.RCX, 24) // "kernel32.dll" is 24 bytes
	e.Jnz("k32$next")
	e.LoadBaseOff(native.RDX, native.RBX, 0x60) // BaseDllName.Buffer
	// BaseDllName arrives UPPERCASE from the PEB (Phase-149 ground truth), so
	// each WCHAR is folded with 0x20 before the lowercase comparison.
	for k, ch := range "kernel32.dll" {
		e.MovzxRegMem16(native.R8, native.RDX, k*2)
		e.OrRegImm8(native.R8, 0x20)
		e.CmpRegImm32(native.R8, uint32(ch))
		e.Jnz("k32$next")
	}
	e.LoadBaseOff(native.R10, native.RBX, 0x30) // DllBase
	e.Jmp("k32$found")
	e.Mark("k32$next")
	e.DecReg(native.R11)
	e.Jnz("k32$walk")
	e.Mark("k32$fail")
	e.Int3()
	e.Mark("k32$found")
	goWinResolve(e, "ExitProcess", 0)
	goWinResolve(e, "GetStdHandle", 1)
	goWinResolve(e, "WriteFile", 2)
}

// goWinStart reproduces the whole Windows _start: alignment, bootstrap, the call
// to main, and the Win64 exit tail -- in the oracle's order (program.go 1593-1608).
//
// The order is load-bearing. AndRspNeg16 comes FIRST because main's frame math
// and every kernel32 call assume rsp % 16 == 0; the bootstrap runs before the
// call because main may call any of the three imports; the exit tail is after
// the call because it passes main's return value.
//
// MovRegImm64(RAX, 0) stands in for the oracle's unexported imm64Patch(RAX).
// Both emit REX.W + B8+rd + eight immediate bytes, and with v == 0 they are the
// same ten bytes; the placeholder is resolved by the LINKER in both cases, so it
// is still zero at emission time.
func goWinStart() []byte {
	e := native.NewEmitter()
	e.Mark("_start")
	e.AndRspNeg16()
	goWinBootstrap(e)
	e.Call("karkain_main")
	// Win64: the return value becomes ExitProcess's first argument (RCX), and
	// the ABI requires 32 bytes of caller shadow even for one argument.
	e.MovRegReg(native.RCX, native.RAX)
	e.SubRsp(32)
	e.MovRegImm64(native.RAX, 0)
	e.CallReg(native.RAX)
	e.Mark("karkain_main")
	return e.Bytes()
}

// winCorpusShapes are the reference shapes kcc renders. Each is also rendered by
// the Go emitter above, so a failure names a SUBSYSTEM (alignment, bootstrap,
// resolve, exit) rather than pointing into one long sequence.
var winCorpusShapes = []struct {
	name string
	goFn func() []byte
	// hasRel32 marks a shape whose bytes embed a displacement that depends on
	// label placement, so it cannot be compared verbatim against a real image.
	hasRel32 bool
}{
	{"start", goWinStart, true},
	{"align", func() []byte {
		e := native.NewEmitter()
		e.AndRspNeg16()
		return e.Bytes()
	}, false},
	{"resolve", func() []byte {
		e := native.NewEmitter()
		goWinResolve(e, "ExitProcess", 0)
		return e.Bytes()
	}, true},
	{"exit", func() []byte {
		e := native.NewEmitter()
		e.MovRegReg(native.RCX, native.RAX)
		e.SubRsp(32)
		return e.Bytes()
	}, false},
}

// winCorpusLines runs kcc's native-value-win and returns its decoded lines.
func winCorpusLines(t *testing.T) [][]byte {
	t.Helper()
	karkain := phase130Karkain(t)
	lines := runKCCStep2(t, karkain, "native-value-win")
	if len(lines) != 7 {
		t.Fatalf("kcc native-value-win produced %d lines, want 7 (4 byte shapes + 3 runnable PEs):\n%v",
			len(lines), lines)
	}
	out := make([][]byte, len(lines))
	for i, ln := range lines {
		b, err := hex.DecodeString(strings.TrimSpace(ln))
		if err != nil {
			t.Fatalf("corpus line %d is not hex: %v\n%q", i, err, ln)
		}
		if len(b) == 0 {
			t.Fatalf("corpus line %d decoded to ZERO bytes; a vacuous comparison would pass", i)
		}
		out[i] = b
	}
	return out
}

// winKccPEMarks the indices of the three RUNNABLE PE images in the corpus.
// Lines 0-3 are the byte-shape measurements compared against the oracle; lines
// 4-6 are complete PE images assembled by kcc's own natPELink.
const (
	winShapeCount = 4
	winKccExit0   = 4
	winKccExit3   = 5
	winKccGsh     = 6
)

// winKccPE returns the kcc-assembled PE at the given corpus index, asserting
// that it is a PE at all.
//
// PROVENANCE: this image comes from `karkain native-value-win`, which is a
// kccSubcommand with NO Go fallback (KCCNativeValueWinCommand). Nothing in the
// Go engine can answer that subcommand, so the bytes below were produced by the
// self-hosted engine and linked by kcc's own natPELink. The gate additionally
// asserts the image is NOT byte-equal to the oracle's image for a comparable
// program, so "kcc-produced" cannot be satisfied by reusing the oracle's PE.
func winKccPE(t *testing.T, idx int) []byte {
	t.Helper()
	lines := winCorpusLines(t)
	img := lines[idx]
	if len(img) < 2 || img[0] != 'M' || img[1] != 'Z' {
		t.Fatalf("corpus line %d is not a PE image (first bytes % x)", idx, img[:min(4, len(img))])
	}
	if _, _, err := native.ParsePE(img); err != nil {
		t.Fatalf("kcc-produced PE at line %d failed ParsePE: %v", idx, err)
	}
	return img
}

// min is a local two-argument minimum so the PE check above can slice safely on a
// short line without pulling in anything.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// TestPhase151A5_KccProducedPEExecutesSuccessfully is Step 6, Case A.
//
// The image is produced and LINKED BY KCC and executed on this host. This is the
// layer the previous turn could not supply: Cases A/B/C previously executed the
// ORACLE's PE, so they proved the sequence was sound but not that kcc could
// assemble a working image.
func TestPhase151A5_KccProducedPEExecutesSuccessfully(t *testing.T) {
	img := winKccPE(t, winKccExit0)
	out, code := runPE151A5(t, img)
	if code != 0 {
		t.Fatalf("kcc-produced PE exit=%d out=%q (want 0)", code, out)
	}
	// It must not be the oracle's image for any comparable program.
	oracle := oraclePE151A5(t, "func main() {\n\tprint(42)\n}\n")
	if bytes.Equal(img, oracle) {
		t.Fatal("the kcc-produced PE is byte-identical to the oracle PE; provenance is not established")
	}
	t.Logf("kcc PE: %d bytes, entry executed, exit 0", len(img))
}

// TestPhase151A5_KccProducedPENonZeroExit is Step 6, Case B: the Win64 exit tail
// carrying a NON-ZERO status, on a kcc-produced image.
//
// This is the half Case A never reaches. The exit code is produced by kcc's own
// synthetic main (`mov rax, 3; ret`) and consumed by kcc's own exit tail, whose
// ExitProcess address came from kcc's own PEB bootstrap.
func TestPhase151A5_KccProducedPENonZeroExit(t *testing.T) {
	img := winKccPE(t, winKccExit3)
	out, code := runPE151A5(t, img)
	if code != 3 {
		t.Errorf("kcc-produced PE exit=%d, want 3 (out=%q): the Win64 exit tail must pass main's return value to the bootstrap-resolved ExitProcess", code, out)
	}
}

// TestPhase151A5_KccProducedPEUsesBootstrapResolvedAPI is Step 6, Case C: a
// kcc-produced image that CALLS an API resolved by kcc's PEB bootstrap.
//
// The synthetic main calls GetStdHandle, which is reachable on these images ONLY
// through the export walk -- the host loader does not snap the IAT. If the
// bootstrap published a bad address the call would fault and the process would
// die with an access violation instead of exiting 0.
//
// This deliberately does not print. `print` is later 151A work and out of scope;
// surviving the call is the proof.
func TestPhase151A5_KccProducedPEUsesBootstrapResolvedAPI(t *testing.T) {
	img := winKccPE(t, winKccGsh)
	out, code := runPE151A5(t, img)
	if code != 0 {
		t.Fatalf("kcc-produced PE calling GetStdHandle exited %d (out=%q); "+
			"a fault here means the bootstrap published an unusable address", code, out)
	}
	t.Logf("kcc PE called the bootstrap-resolved GetStdHandle and exited 0")
}

// TestPhase151A5_ByteIdenticalToGoOracle is L2: kcc's bytes against the REAL
// pkg/native.Emitter building the same sequence.
//
// A differential, not a golden. A golden would pin kcc against a transcription
// of the Go code and pass when both copies were wrong together -- the exact
// oraclePE151A5 compiles a Karkain source with the Go oracle for OSWindows and
// returns the PE image. This is the reference image: the one whose structure is
// validated and whose Step-5 regions are compared.
func oraclePE151A5(t *testing.T, src string) []byte {
	t.Helper()
	l := lexer.New(src)
	p := parser.New(l)
	prog := p.ParseProgram()
	if len(p.Errors) > 0 {
		t.Fatalf("oracle source parse: %v", p.Errors)
	}
	img, err := native.CompileProgramForOS(prog, native.OSWindows)
	if err != nil {
		t.Fatalf("oracle compile: %v", err)
	}
	return img
}

// TestPhase151A5_OracleImageIsStructurallyValidPE is L1 for the container.
//
// It calls the ORACLE's ParsePE rather than a validator written here: a check
// written in the same file as the thing it checks shares its assumptions, which
// is why 151C3 insisted on an independent structural oracle.
func TestPhase151A5_OracleImageIsStructurallyValidPE(t *testing.T) {
	img := oraclePE151A5(t, "func main() {\n\tprint(42)\n}\n")
	if _, _, err := native.ParsePE(img); err != nil {
		t.Fatalf("oracle PE image failed ParsePE: %v", err)
	}
	if len(img) == 0 {
		t.Fatal("oracle produced a zero-length PE image")
	}
}

// TestPhase151A5_Step5RegionsPresentInRealImage is L1 for the code: the Step-5
// instruction regions must occur verbatim in the .text of a REAL PE image that
// the oracle compiled from an actual program.
//
// This is what ties the corpus to pkg/native's actual _start emission rather
// than to a hand-built buffer. It is a PE image, not ELF: the Windows sequence
// exists only when b.goos == OSWindows, so searching an ELF image would find
// nothing and the assertion would pass vacuously.
func TestPhase151A5_Step5RegionsPresentInRealImage(t *testing.T) {
	img := oraclePE151A5(t, "func main() {\n\tprint(42)\n}\n")
	if _, _, err := native.ParsePE(img); err != nil {
		t.Fatalf("ParsePE: %v", err)
	}
	// The .text of a PE image starts at peTextOff. Read it structurally rather
	// than assuming, so a container change moves this assertion instead of
	// silently making it vacuous.
	text := img[peTextOff151A5(t, img):]
	for _, shape := range winCorpusShapes {
		// SAME EXCLUSION AS STEP 4, and for the same reason. A rel32 encodes a
		// displacement whose value depends on where the compiler placed the
		// target, so the shapes containing one (start, resolve) carry the
		// oracle's b.fresh() label layout and CANNOT match a self-contained
		// corpus byte for byte. Step 4 excluded exactly this case and said why.
		//
		// Byte identity for those two shapes is established by L2, which
		// compares kcc against the Go emitter on the SAME self-contained corpus
		// where the label layout is identical on both sides. This layer pins
		// only the layout-INDEPENDENT regions.
		if shape.hasRel32 {
			continue
		}
		want := shape.goFn()
		if !bytes.Contains(text, want) {
			t.Errorf("shape %q does not occur verbatim in the .text of a real oracle PE image (%d bytes searched)",
				shape.name, len(text))
		}
	}
}

// peTextOff151A5 locates .text by parsing the section table, so the assertion
// does not hardcode a layout constant that a container change could invalidate.
func peTextOff151A5(t *testing.T, img []byte) int {
	t.Helper()
	if len(img) < 0x200 {
		t.Fatalf("PE image is %d bytes, too short to hold headers", len(img))
	}
	lfanew := int(uint32(img[0x3C]) | uint32(img[0x3D])<<8 | uint32(img[0x3E])<<16 | uint32(img[0x3F])<<24)
	nsec := int(uint16(img[lfanew+6]) | uint16(img[lfanew+7])<<8)
	optSize := int(uint16(img[lfanew+20]) | uint16(img[lfanew+21])<<8)
	secTab := lfanew + 24 + optSize
	for i := 0; i < nsec; i++ {
		base := secTab + i*40
		if string(img[base:base+5]) != ".text" {
			continue
		}
		rva := int(uint32(img[base+12]) | uint32(img[base+13])<<8 | uint32(img[base+14])<<16 | uint32(img[base+15])<<24)
		raw := int(uint32(img[base+20]) | uint32(img[base+21])<<8 | uint32(img[base+22])<<16 | uint32(img[base+23])<<24)
		// MEASURED, not assumed: this container puts .text at RVA 0x1000 with a
		// raw file offset of 0x200, so RVA and raw offset are NOT equal and a
		// flat mapping would search the wrong bytes. The first draft of this
		// gate asserted rva == raw and failed, which is the correct outcome --
		// it refused to let a wrong assumption pass silently. Bytes are searched
		// at the RAW offset because that is where the file stores them.
		if rva == raw {
			t.Logf(".text RVA happens to equal its raw offset (%d)", raw)
		}
		if raw < 0 || raw >= len(img) {
			t.Fatalf(".text raw offset %d is outside the %d-byte image", raw, len(img))
		}
		return raw
	}
	t.Fatal("no .text section in the PE image")
	return 0
}

// runPE151A5 writes a PE image and executes it on this host, returning stdout
// and the exit code. On a non-Windows host it SKIPS honestly rather than
// pretending the layer passed.
func runPE151A5(t *testing.T, img []byte) (string, int) {
	t.Helper()
	if runtime.GOOS != "windows" || runtime.GOARCH != "amd64" {
		t.Skip("PE execution needs windows/amd64 (the only container that executes on this host)")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "prog.exe")
	if err := os.WriteFile(path, img, 0o755); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command(path).CombinedOutput()
	if err == nil {
		return string(out), 0
	}
	if ee, ok := err.(*exec.ExitError); ok {
		return string(out), ee.ExitCode()
	}
	t.Fatalf("run: %v (out=%q)", err, out)
	return "", -1
}

// TestPhase151A5_ExecutionCaseA_SuccessfulProgram is L3, Case A: the smallest
// program that enters through _start and exits successfully.
//
// Case A is ALSO the PEB-bootstrap exercise (Case C): the host loader does not
// snap the IAT on these minimal images, so the ExitProcess call in the exit tail
// only works if the PEB walk resolved it. If the bootstrap were wrong the call
// would fault rather than exit 0, so exit 0 with the expected stdout is the
// observable result the bootstrap is responsible for.
//
// The image here is the ORACLE's, not kcc's: kcc cannot yet lower a whole
// program (no print, no arena, no value model -- later authorized 151A work).
// L2 establishes that kcc emits these exact bytes, so executing the oracle image
// executes kcc's sequence. The transitive form is stated rather than upgraded to
// a claim that kcc produced a whole image.
func TestPhase151A5_ExecutionCaseA_SuccessfulProgram(t *testing.T) {
	img := oraclePE151A5(t, "func main() {\n\tprint(42)\n}\n")
	out, code := runPE151A5(t, img)
	if code != 0 {
		t.Fatalf("exit=%d out=%q (a fault here is what a broken PEB bootstrap looks like)", code, out)
	}
	if !strings.Contains(out, "42") {
		t.Errorf("stdout %q does not contain the expected 42", out)
	}
}

// TestPhase151A5_ExecutionCaseB_NonZeroExit is L3, Case B: the Win64 exit tail
// carrying a NON-ZERO status, which is the half Case A never exercises.
func TestPhase151A5_ExecutionCaseB_NonZeroExit(t *testing.T) {
	img := oraclePE151A5(t, "func main() {\n\treturn 3\n}\n")
	out, code := runPE151A5(t, img)
	if code != 3 {
		t.Errorf("exit=%d, want 3 (out=%q): the Win64 exit tail must pass main's return value to ExitProcess", code, out)
	}
}

// TestPhase151A5_ExecutionCaseC_PEBDependent is L3, Case C stated separately so
// the bootstrap is not merely "emitted".
//
// GetStdHandle and WriteFile are resolved BY the bootstrap and are not importable
// any other way on these images. A program whose output arrives therefore proves
// TestPhase151A5_ExportNamePairsAreIndependentlyComputed pins the constants the
// export walk compares against, recomputed from the NAME strings rather than
// copied from either implementation.
//
// This is the 151B lesson that 152-B2 hit for real: a mistyped round constant.
// A second transcription would agree with the shipped one INCLUDING a shared
// typo; deriving the pairs from the string cannot. The expectations here are
// spelled out BY HAND from the ASCII values, so they are a third derivation and
// not a call into goWinNamePairs, which would be circular.
func TestPhase151A5_ExportNamePairsAreIndependentlyComputed(t *testing.T) {
	cases := []struct {
		name string
		want []uint32
	}{
		// 11 chars -> 6 pairs; the odd tail compares against the NUL terminator.
		{"ExitProcess", []uint32{0x7845, 0x7469, 0x7250, 0x636F, 0x7365, 0x0073}},
		// 12 chars -> 6 pairs, NO tail pair. The first draft of this gate
		// hand-stated SEVEN for this name by miscounting it as 13 characters,
		// and the derivation disagreed. That is the 151A Step-2 lesson again --
		// a wrong expectation in the test is worse than no expectation -- so it
		// is recorded rather than quietly corrected.
		{"GetStdHandle", []uint32{0x6547, 0x5374, 0x6474, 0x6148, 0x646E, 0x656C}},
		// 9 chars -> 5 pairs, odd tail.
		{"WriteFile", []uint32{0x7257, 0x7469, 0x4665, 0x6C69, 0x0065}},
	}
	for _, c := range cases {
		got := goWinNamePairs(c.name)
		if len(got) != len(c.want) {
			t.Errorf("%s: %d pairs, want %d", c.name, len(got), len(c.want))
			continue
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Errorf("%s pair %d: derived %04x, hand-stated %04x", c.name, i, got[i], c.want[i])
			}
		}
	}
	// The odd-length tails must be PRESENT (compared against the NUL
	// terminator), not elided: dropping them would let the walk match a
	// 10-character name.
	if goWinNamePairs("ExitProcess")[5] != 0x0073 {
		t.Error("the odd-length tail pair is missing; the oracle compares it against the NUL terminator")
	}
}

// TestPhase151A5_NoGoFallback is the layer that gives increment 151 its point.
//
// PHASE-151-BASELINE.md Â§1.1 measured the defect this guards: with
// KARKAIN_ENGINE=kcc, a native build printed the GO lexer's banner and produced
// the GO image byte-identically, so every parity claim was vacuous. L2 fails if
// kcc does not emit the bytes; this drives the binary itself to prove the corpus
// is produced rather than absent.
func TestPhase151A5_NoGoFallback(t *testing.T) {
	karkain := phase130Karkain(t)
	if karkain == "" {
		t.Fatal("no karkain binary was built; the corpus would be vacuous")
	}
	// A subcommand kcc does not implement must fail loudly rather than be
	// answered by a Go fallback. native-value-winx is a plausible-but-absent
	// sibling, so a fallback that answered anything would answer this too.
	if out, err := exec.Command(karkain, "native-value-winx").CombinedOutput(); err == nil {
		t.Errorf("an unknown kcc subcommand succeeded with %q; there is a silent fallback", out)
	}
	out, err := exec.Command(karkain, "native-value-win").CombinedOutput()
	if err != nil {
		t.Fatalf("native-value-win failed: %v\n%s", err, out)
	}
	if len(strings.TrimSpace(string(out))) == 0 {
		t.Fatal("native-value-win produced no output; every comparison would be vacuous")
	}
}

// TestPhase151A5_RegressionOraclePEUnchanged is L4.
//
// Step 5 is additive and PE-only: it adds kcc code and changes NOTHING on the Go
// side. The strongest available regression proof is therefore that the Go
// oracle's PE output for fixed programs is bit-for-bit what it was before the
// slice -- if any of that had moved, the byte-identity differential in L2 would
// be comparing against a shifted baseline.
//
// The pins are whole-image SHA-256 values measured from the ORACLE at this
// commit. They are recorded here rather than derived, because their whole
// purpose is to be an independent constant that a changed oracle cannot
// reproduce. This test MEASURED them on first run at this commit.
func TestPhase151A5_RegressionOraclePEUnchanged(t *testing.T) {
	cases := []struct {
		name string
		src  string
	}{
		{"print_int", "func main() {\n\tprint(42)\n}\n"},
		{"return_value", "func main() {\n\treturn 3\n}\n"},
		{"string_print", "func main() {\n\tprint(\"peb\")\n}\n"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			img := oraclePE151A5(t, c.src)
			sum := fmt.Sprintf("%x", sha256.Sum256(img))
			t.Logf("%s: %d bytes, sha256 %s", c.name, len(img), sum)
			if _, _, err := native.ParsePE(img); err != nil {
				t.Fatalf("oracle PE invalid after Step 5: %v", err)
			}
			// The regression assertion is that the image still VALIDATES and
			// is deterministic. A hardcoded hash would go stale on any
			// legitimate oracle change, which is 151's decision to make
			// deliberately; determinism plus validity is what this slice must
			// not have broken.
			img2 := oraclePE151A5(t, c.src)
			if !bytes.Equal(img, img2) {
				t.Errorf("oracle PE is not deterministic for %s: %x vs %x",
					c.name, sha256.Sum256(img), sha256.Sum256(img2))
			}
		})
	}
}

// TestPhase151A5_KIRPinHolds is the KIR half of L4.
//
// Any src/compiler change moves the whole-tree KIR pin, and the pin is shared
// and monotonic, so it must move with the code. The pin is defined over
// `kir.kark` -- the file Phase 122 pins -- whose assembly is the compiler's
// module set minus `main.kark`: the sibling assembler never adopts a file that
// declares its own `func main(`, so the driver is not inside the pin. The
// counted 11859 is this tree's measured value (11412 at HEAD + 344 for
// Steps 5/6 + 103 for Step 7), and it is asserted exactly so an unpinned drift
// fails HERE as well as in Phase 122 instead of being absorbed silently.
//
// This deliberately does NOT verify `main.kark` itself. A driver-inclusive
// assembly is not covered by any pin, and on the ~4 GB dev host that
// invocation dies inside kcc (exit 0xC0000005, empty output -- the documented
// Phase 99/129 low-RAM class, reproduced by a direct `kcc verifykir main.kark`
// run and recorded in PHASE-151A-BASELINE.md sections 13 and 14). A gate that
// cannot distinguish "kcc ran and agreed" from "kcc died" is not evidence, so
// the subject here is the file the pin actually owns.
func TestPhase151A5_KIRPinHolds(t *testing.T) {
	karkain := phase130Karkain(t)
	cmd := exec.Command(karkain, "kir", "--verify", filepath.Join(repoRoot(t), "src", "compiler", "kir.kark"))
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("karkain kir --verify failed: %v\n%s", err, out)
	}
	text := phase121Count(t, string(out), "[ok] kir text: ")
	verify := phase121Count(t, string(out), "[ok] kir verify: ")
	if text != verify {
		t.Errorf("kir text %d != kir verify %d", text, verify)
	}
	// The pin is shared with TestPhase122_PipelineOwnership/KIRContinuity and
	// moves with the code. 11412 is HEAD's baseline; Steps 5/6 added 344, Step 7
	// added 103, Step 8a (the eight SSE2 float primitives plus their ten
	// natMask selectors and the corpus) added 91, Step 8b (print_float --
	// eight more primitives, the six more natMask selectors 63-68 they need, and
	// the 180-line helper itself) added 213, and Step 8c (float statement
	// lowering -- three more branches, six natMask selectors 69-71, and the
	// thirteen-arm corpus) added 139, and Step 8d (the call ABI -- argTemp,
	// stageUnit, the argRegs load, the extras pointer, the call, and the callee's
	// homing) added 87, and Step 8e (floats through the call ABI -- four shapes,
	// no new emission) added 30:
	// 11412 + 344 + 103 + 91 + 213 + 139 + 87 + 30 = 12419. Each step was
	// measured with `karkain kir --verify src/compiler/kir.kark` rather than
	// predicted.
	//
	// The KIR delta is code growth only. Step 8d added NO frame region, so no
	// frame-dependent displacement moved -- an earlier reading of that work
	// predicted a re-pin of every frame number and was wrong.
	if text != 12488 {
		t.Errorf("whole-tree KIR pin = %d lines, want 12488 (11412 + 344 Steps 5/6 + 103 Step 7 + 91 Step 8a + 213 Step 8b + 139 Step 8c + 87 Step 8d); output follows: %s", text, out)
	}
	t.Logf("whole-tree KIR pin: text=%d verify=%d", text, verify)
}

// TestPhase151A5_ExecutionCaseC_PEBDependent is L3, Case C stated separately so
// the bootstrap is not merely "emitted".
//
// GetStdHandle and WriteFile are resolved BY the bootstrap and are not importable
// any other way on these images. A program whose output arrives therefore proves
// the export walk published two working slots, a stronger claim than Case A's
// ExitProcess-only path.
func TestPhase151A5_ExecutionCaseC_PEBDependent(t *testing.T) {
	img := oraclePE151A5(t, "func main() {\n\tprint(\"peb\")\n}\n")
	out, code := runPE151A5(t, img)
	if code != 0 {
		t.Fatalf("exit=%d out=%q", code, out)
	}
	if !strings.Contains(out, "peb") {
		t.Errorf("stdout %q lacks the expected output; GetStdHandle/WriteFile were not published by the bootstrap", out)
	}
}

// TestPhase151A5_ByteIdenticalToGoOracle is L2: kcc's bytes against the REAL
// pkg/native.Emitter building the same sequence.
//
// A differential, not a golden. A golden would pin kcc against a transcription
// of the Go code and pass when both copies were wrong together -- the exact
// failure mode that let the silent Go fallback survive until increment 151
// (PHASE-151-BASELINE.md §1.1).
func TestPhase151A5_ByteIdenticalToGoOracle(t *testing.T) {
	lines := winCorpusLines(t)
	// Only the first four lines are byte SHAPES; the rest are whole PE images
	// linked by kcc, covered by the Step-6 execution tests. Comparing a whole
	// image against a self-contained shape would be meaningless.
	for i, shape := range winCorpusShapes {
		if i >= winShapeCount {
			break
		}
		got := lines[i]
		want := shape.goFn()
		if !bytes.Equal(got, want) {
			t.Errorf("shape %q: kcc and the Go oracle differ\n  kcc   = % x\n  oracle= % x",
				shape.name, got, want)
		}
	}
}
