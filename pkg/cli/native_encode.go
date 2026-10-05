package cli

import (
	"io"
	"strconv"
	"strings"
)

// KCCNativeEncodeCommand runs the self-hosted engine's machine-code encoder
// over its reference instruction sequences and returns the bytes as hex.
//
// This is increment 151B's measurement surface, and it exists for the same
// reason `kir` exists: the component under test lives in the SELF-HOSTED
// engine (src/compiler/native_emit.kark), so the Go side must not
// reimplement it. There is deliberately no Go fallback here. A Go encoder
// that produced the same bytes would make the gate pass while proving
// nothing about kcc, which is precisely the dishonesty increment 151 was
// opened to remove.
//
// The subcommand takes no input file: the sequences are compiled into kcc, so
// a differential against the Go oracle (pkg/native.Emitter) compares the two
// implementations directly rather than comparing two invocations of one.
func KCCNativeEncodeCommand(w io.Writer, verbose bool) CommandResult {
	bin, err := kccBinaryPath(w)
	if err != nil {
		// kcc unavailable is an ENVIRONMENT failure, not a compile failure.
		// Reporting it as a compile error would blame the user's program for
		// a missing toolchain.
		return CommandResult{ExitCode: ExitEnv, Message: err.Error()}
	}

	out, code := runKCC(bin, "native-encode", "")

	// The encoder's two refusal cases print error[K117] on stdout as part of
	// its normal output; they are data, not a build failure. A non-zero exit
	// from kcc therefore still has to be surfaced, but the presence of the
	// lines must not be turned into a diagnostic.
	var lines []string
	for _, ln := range strings.Split(strings.TrimRight(out, "\n"), "\n") {
		ln = strings.TrimRight(ln, "\r")
		if strings.TrimSpace(ln) == "" {
			continue
		}
		lines = append(lines, ln)
	}

	msg := strings.Join(lines, "\n")
	if code != 0 && msg == "" {
		msg = "kcc native-encode failed with exit " + strconv.Itoa(code)
	}
	return CommandResult{ExitCode: code, Message: msg}
}

// KCCNativeELFCommand runs the self-hosted ELF64 container writer over its
// reference cases and returns the images as hex.
//
// Increment 151C's measurement surface, and it works exactly like
// native-encode's: the cases are compiled into kcc, the Go side never
// reimplements the writer, and the gate compares kcc's images against
// pkg/native's Link.
//
// The container CAN be delivered before the value model, and that is the
// reason this increment is tractable: Link takes already-encoded bytes
// (text, rodata, data) plus two offsets and knows nothing about functions,
// arrays or strings, so it is pure serialisation and can be proven against
// fixed input bytes.
func KCCNativeELFCommand(w io.Writer, verbose bool) CommandResult {
	return kccSubcommand(w, "native-elf")
}

// KCCNativeMachOCommand runs the self-hosted Mach-O PIE container writer over
// its reference cases and returns the images as hex.
//
// Increment 151C2, same contract as the other two container commands. Mach-O
// is the subtle one of the three: its load-command chain is sized by its own
// contents, and the rebase opcode stream is what makes the image a real PIE,
// so the writer and the stream encoder are entangled rather than separable.
func KCCNativeMachOCommand(w io.Writer, verbose bool) CommandResult {
	return kccSubcommand(w, "native-macho")
}

// KCCNativePECommand runs the self-hosted PE32+ container writer over its
// reference cases and returns the images as hex.
//
// Increment 151C3, same contract as the other three. PE is the only one of
// the four containers that a Windows loader will accept and RUN, which is
// exactly why the structural evidence here is worth as much as it is: the
// earlier native images are executed by the increment 145-150 PE gates, and
// those images were produced by the Go writer.
func KCCNativePECommand(w io.Writer, verbose bool) CommandResult {
	return kccSubcommand(w, "native-pe")
}

// KCCNativeValueCommand runs the self-hosted native value/frame foundation
// (increment 151A Step 1, src/compiler/native_value.kark) over its two
// reference programs and returns the machine code as hex, one line each.
//
// Same contract as the four above: the cases are compiled into kcc, the Go
// side never reimplements the layout, and the gate compares kcc's bytes
// against pkg/native, which is the oracle. The reference programs are
// deliberately helper-free -- no print, no arena, no entry stub -- because
// those belong to later 151A slices and depending on them here would make
// this slice's evidence unreachable.
func KCCNativeValueCommand(w io.Writer, verbose bool) CommandResult {
	return kccSubcommand(w, "native-value")
}

// KCCNativeValueLayoutCommand returns the frame arithmetic as text.
//
// The bytes alone are not sufficient evidence for a frame layout: if kcc and
// the oracle computed the SAME wrong offset, their bytes would still agree.
// This surface exposes the offsets themselves so the gate can compare them
// against values derived independently in the test.
func KCCNativeValueLayoutCommand(w io.Writer, verbose bool) CommandResult {
	return kccSubcommand(w, "native-value-layout")
}

// KCCNativeValuePrimCommand renders each encoder opcode this slice added on
// its own, so a mismatch names the primitive instead of pointing at a whole
// function body. Compared three ways in the gate: against the Go oracle's
// identically named Emitter method, and against the bytes stated from the
// Intel SDM.
func KCCNativeValuePrimCommand(w io.Writer, verbose bool) CommandResult {
	return kccSubcommand(w, "native-value-prims")
}

// KCCNativeValueIntCommand is increment 151A Step 2: the self-hosted integer
// statement/expression lowering. Returns the machine code of the
// straight-line integer reference programs, one line each.
//
// The surface is the ORACLE's, not the language's. The Go implementation
// lowers only +, - and * for integers and refuses everything else, and a
// comparison is a branch rather than a value; those limits are reproduced
// rather than widened, so this command is deliberately smaller than the
// language's integer surface.
func KCCNativeValueIntCommand(w io.Writer, verbose bool) CommandResult {
	return kccSubcommand(w, "native-value-int")
}

// KCCNativeValueCmpCommand returns the six integer comparisons, one program
// per operator, so a mismatch names the operator whose jump opcode is wrong.
func KCCNativeValueCmpCommand(w io.Writer, verbose bool) CommandResult {
	return kccSubcommand(w, "native-value-cmp")
}

// KCCNativeValueRefuseCommand returns the integer diagnostics the oracle
// raises for constructs it does not lower. These are DATA, not build
// failures: the gate asserts kcc refuses exactly what the oracle refuses, so
// narrowing the surface must stay visible.
func KCCNativeValueRefuseCommand(w io.Writer, verbose bool) CommandResult {
	return kccSubcommand(w, "native-value-refuse")
}

// KCCNativeValueLoopCommand renders the ten control-flow reference programs:
// simple/arithmetic while, while with break, while with continue, simple and
// full C-style for, for with break, for with continue, nested whiles, and a
// nested for/while pair.
//
// Loops are the first surface whose byte-identity depends on LABEL NAMES and
// not only on opcode bytes: a rel32 encodes just a displacement, so a
// renumbered label can produce a coincidentally equal byte stream.
func KCCNativeValueLoopCommand(w io.Writer, verbose bool) CommandResult {
	return kccSubcommand(w, "native-value-loop")
}

// KCCNativeValueLoopLabelsCommand reports the fresh() label names each
// control-flow reference program uses, in allocation order. This is the layer
// that catches a wrong label counter even when the displacements happen to
// come out equal.
func KCCNativeValueLoopLabelsCommand(w io.Writer, verbose bool) CommandResult {
	return kccSubcommand(w, "native-value-loop-labels")
}

// KCCNativeValueStartCommand runs the 151A `_start` measurement: the Linux
// entry stub (call karkain_main, mov rdi,rax, mov rax,60, syscall), the bare
// syscall primitive, and the exit tail without its call.
//
// There is deliberately no Go fallback here. This command exists so the gate
// can compare kcc's bytes against pkg/native's, and a Go-side emitter that
// produced the expected bytes itself would make that comparison vacuous while
// still looking green.
func KCCNativeValueStartCommand(w io.Writer, verbose bool) CommandResult {
	return kccSubcommand(w, "native-value-start")
}

// KCCNativeValueWinCommand runs the 151A Step 5 measurement: the Windows entry
// (`and rsp, -16`), the PEB bootstrap with its three export resolves, one export
// resolve alone, and the Win64 exit tail.
//
// There is deliberately no Go fallback here, for the reason
// KCCNativeValueStartCommand records: a Go-side emitter producing the expected
// bytes would make the oracle comparison vacuous while still looking green.
func KCCNativeValueWinCommand(w io.Writer, verbose bool) CommandResult {
	return kccSubcommand(w, "native-value-win")
}

// KCCNativeValuePrintCommand runs the 151A Step 7 measurement: the int `print`
// helper, its digit loop alone, and a bare rodata reference.
//
// There is deliberately no Go fallback here, for the reason
// KCCNativeValueStartCommand records: a Go-side emitter producing the expected
// bytes would make the oracle comparison vacuous while still looking green.
//
// This is the first surface that carries UNRESOLVED absolute-address
// placeholders rather than pure instruction bytes, because a rodata reference
// is only known at link time. That is why one of the three arms exists on its
// own: the placeholder shape is pinned separately from the helper that uses it.
func KCCNativeValuePrintCommand(w io.Writer, verbose bool) CommandResult {
	return kccSubcommand(w, "native-value-print")
}

// KCCNativeValueFloatCommand runs the 151A Step 8a measurement: the eight SSE2
// scalar-double primitives, in four sequences chosen to cover every distinct
// encoding decision rather than one per primitive.
//
// Same no-Go-fallback contract as every other command in this file. The
// sequences are compiled into kcc; the Go side only runs it.
func KCCNativeValueFloatCommand(w io.Writer, verbose bool) CommandResult {
	return kccSubcommand(w, "native-value-float")
}

// KCCNativeValueFloatStmtCommand runs the 151A Step 8c measurement: float
// statement lowering, which is the first slice where an expression lowering
// exists and the Step 8a primitives plus the Step 8b helper become reachable
// from actual code.
//
// Thirteen arms: the literal store, the local reload, `+ - * /`, unary
// negation, and the six comparisons. Same no-Go-fallback contract.
func KCCNativeValueFloatStmtCommand(w io.Writer, verbose bool) CommandResult {
	return kccSubcommand(w, "native-value-floatstmt")
}

// KCCNativeValueCallCommand runs the 151A Step 8d measurement: the call ABI,
// int first.
//
// Seven shapes: calls with one, two and three register-budget arguments, a
// seven-argument call that spills unit 6 into the extras area and materialises
// R10, and the callee's homing for one, three and seven parameters. Same
// no-Go-fallback contract.
func KCCNativeValueCallCommand(w io.Writer, verbose bool) CommandResult {
	return kccSubcommand(w, "native-value-call")
}

// kccSubcommand runs one read-only kcc measurement subcommand and returns its
// non-empty output lines as a single message.
//
// It is shared by all three because they are genuinely the same shape: no
// input file, no fallback, exit code and diagnostics passed through. The one
// thing it deliberately does NOT do is interpret the output -- a refusal line
// like "error[K117] ..." is DATA here, not a build failure, because the
// reference corpus deliberately includes refusal cases.
func kccSubcommand(w io.Writer, sub string) CommandResult {
	bin, err := kccBinaryPath(w)
	if err != nil {
		// kcc unavailable is an ENVIRONMENT failure, not a compile failure.
		// Reporting it as a compile error would blame the user's program for
		// a missing toolchain.
		return CommandResult{ExitCode: ExitEnv, Message: err.Error()}
	}
	out, code := runKCC(bin, sub, "")
	var lines []string
	for _, ln := range strings.Split(strings.TrimRight(out, "\n"), "\n") {
		ln = strings.TrimRight(ln, "\r")
		if strings.TrimSpace(ln) == "" {
			continue
		}
		lines = append(lines, ln)
	}
	msg := strings.Join(lines, "\n")
	if code != 0 && msg == "" {
		msg = "kcc " + sub + " failed with exit " + strconv.Itoa(code)
	}
	return CommandResult{ExitCode: code, Message: msg}
}
