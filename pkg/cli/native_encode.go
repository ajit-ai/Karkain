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
