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
