// Package bootstrap implements a multi-stage self-hosting bootstrap pipeline
// for the Karkain compiler. It executes a 3-stage compilation where each
// successive compiler binary is produced by the previous one, culminating
// in a bitwise-identical stage2 == stage3 proof of self-hosting correctness.
package bootstrap

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"
)

// StageResult holds results for a single compilation stage.
type StageResult struct {
	Stage    int
	Binary   string
	Size     int64
	SHA256   string
	Duration time.Duration
}

func binDir(projectRoot string) string {
	return filepath.Join(projectRoot, "bin")
}

func exeName(name string) string {
	if runtime.GOOS == "windows" {
		return name + ".exe"
	}
	return name
}

func binPath(projectRoot, name string) string {
	return filepath.Join(binDir(projectRoot), exeName(name))
}

func fileHash(path string) (string, int64, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", 0, fmt.Errorf("reading %s: %w", path, err)
	}
	h := sha256.Sum256(data)
	return fmt.Sprintf("%x", h), int64(len(data)), nil
}

// reproducibleEpoch is a fixed SOURCE_DATE_EPOCH injected into child processes.
// GNU ld (binutils >= 2.40, as shipped with MSYS2/MinGW gcc 14.x) embeds the
// current wall-clock time into the PE TimeDateStamp of produced binaries unless
// SOURCE_DATE_EPOCH is set, which makes identical-input compiles produce
// byte-different executables. Fixing it to a constant makes the link step fully
// reproducible (stage2 == stage3 bitwise), which is exactly the deterministic
// self-hosting proof the bootstrap pipeline asserts. It is left untouched if the
// caller already exports one.
const reproducibleEpoch = "1072915200" // 2004-01-01T00:00:00Z

func runCmd(dir, name string, args ...string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	setSourceDateEpoch(cmd)
	return cmd.Run()
}

func runCmdOutput(dir, name string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	setSourceDateEpoch(cmd)
	forceGoEngine(cmd)
	return cmd.CombinedOutput()
}

// setSourceDateEpoch injects a fixed SOURCE_DATE_EPOCH into the child process
// environment unless one is already present. This guarantees reproducible
// binaries from gcc/go regardless of the host wall-clock.
func setSourceDateEpoch(cmd *exec.Cmd) {
	for _, kv := range cmd.Env {
		if len(kv) >= 17 && kv[:17] == "SOURCE_DATE_EPOCH=" {
			return
		}
	}
	cmd.Env = append(os.Environ(), "SOURCE_DATE_EPOCH="+reproducibleEpoch)
}

// forceGoEngine pins KARKAIN_ENGINE=go on the compiler-invoking commands. The
// bootstrap pipeline is definitionally the Go bootstrap: stage-1 requires the Go
// front end to emit src/compiler/main.c (kcc both defaults to the self-hosted
// engine since Phase 97 and emits C23 artifacts to a temp sandbox, not the
// source directory). Without this pin, the bootstrap would recurse into kcc and
// never produce the expected C artifact.
func forceGoEngine(cmd *exec.Cmd) {
	cmd.Env = append(cmd.Env, "KARKAIN_ENGINE=go")
}

// RunBootstrap executes the 3-stage bootstrap pipeline.
//
//  1. Go compiler compiles src/compiler/main.kark -> karkain-compiler1
//  2. karkain-compiler1 compiles src/compiler/main.kark -> karkain-compiler2
//  3. karkain-compiler2 compiles src/compiler/main.kark -> karkain-compiler3
//
// Returns Stage1, Stage2, Stage3 results and any error.
func RunBootstrap(projectRoot string) (*StageResult, *StageResult, *StageResult, error) {
	s1, err := RunStage1(projectRoot)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("stage 1 failed: %w", err)
	}

	s2, err := RunStage2(projectRoot, s1.Binary)
	if err != nil {
		return s1, nil, nil, fmt.Errorf("stage 2 failed: %w", err)
	}

	s3, err := RunStage3(projectRoot, s2.Binary)
	if err != nil {
		return s1, s2, nil, fmt.Errorf("stage 3 failed: %w", err)
	}

	return s1, s2, s3, nil
}

// VerifyIdentity checks that two StageResults have identical SHA256 hashes.
func VerifyIdentity(a, b *StageResult) (bool, error) {
	if a == nil || b == nil {
		return false, fmt.Errorf("nil stage result")
	}
	return a.SHA256 == b.SHA256, nil
}

// RunStage1 uses the Go compiler (go build) to produce the Go-based karkain
// binary, then uses it to compile src/compiler/main.kark via C transpilation
// and gcc into karkain-compiler1.
func RunStage1(projectRoot string) (*StageResult, error) {
	start := time.Now()

	bd := binDir(projectRoot)
	if err := os.MkdirAll(bd, 0755); err != nil {
		return nil, fmt.Errorf("creating bin directory: %w", err)
	}

	stage1GoBinary := binPath(projectRoot, "karkain-stage1")
	compiler1Binary := binPath(projectRoot, "karkain-compiler1")
	karSource := filepath.Join(projectRoot, "src", "compiler", "main.kark")

	// Step 1a: go build -o bin/karkain-stage1[.exe] ./cmd/karkain
	fmt.Printf("[Stage 1] Building Go compiler binary: %s\n", stage1GoBinary)
	if err := runCmd(projectRoot, "go", "build", "-o", stage1GoBinary, "./cmd/karkain"); err != nil {
		return nil, fmt.Errorf("go build failed: %w", err)
	}

	// Step 1b: Use Go-based karkain to transpile main.kark -> C
	fmt.Printf("[Stage 1] Transpiling %s -> C\n", karSource)
	output, err := runCmdOutput(projectRoot, stage1GoBinary, "build", karSource, "--target", "c23")
	if err != nil {
		// Log detailed failure reason
		if len(output) > 0 {
			fmt.Printf("[Stage 1] Transpilation failed with output:\n%s\n", string(output))
		}
		return nil, fmt.Errorf("karkain transpile failed: %v: %s", err, string(output))
	}

	// Step 1c: Find the generated C file and compile with gcc
	cFile := findGeneratedCFile(projectRoot, karSource)
	if cFile == "" {
		return nil, fmt.Errorf("generated C file not found after transpilation")
	}
	// Always clean up the generated C file, even if gcc compilation fails, so a
	// stale artifact cannot shadow a freshly generated file in later stages/tests.
	defer os.Remove(cFile)

	fmt.Printf("[Stage 1] Compiling C -> %s\n", compiler1Binary)
	if err := compileWithGCC(cFile, compiler1Binary); err != nil {
		return nil, fmt.Errorf("gcc compilation failed: %w", err)
	}

	sha, size, err := fileHash(compiler1Binary)
	if err != nil {
		return nil, err
	}

	result := &StageResult{
		Stage:    1,
		Binary:   compiler1Binary,
		Size:     size,
		SHA256:   sha,
		Duration: time.Since(start),
	}
	fmt.Printf("[Stage 1] Done: %s (%d bytes, %s, %v)\n", filepath.Base(compiler1Binary), size, sha[:16], result.Duration)
	return result, nil
}

// RunStage2 uses karkain-stage1 (the Go-based compiler) to compile
// src/compiler/main.kark -> C -> gcc -> karkain-compiler2.
func RunStage2(projectRoot string, stage1Binary string) (*StageResult, error) {
	return runCompileStage(projectRoot, 2, "karkain-compiler2", stage1Binary)
}

// RunStage3 uses karkain-compiler2 to compile
// src/compiler/main.kark -> C -> gcc -> karkain-compiler3.
func RunStage3(projectRoot string, stage2Binary string) (*StageResult, error) {
	return runCompileStage(projectRoot, 3, "karkain-compiler3", stage2Binary)
}

// runCompileStage is the common logic for stages 2 and 3.
// It invokes the given compiler binary to transpile main.kark, then compiles with gcc.
func runCompileStage(projectRoot string, stage int, outputName, compilerBinary string) (*StageResult, error) {
	start := time.Now()

	bd := binDir(projectRoot)
	if err := os.MkdirAll(bd, 0755); err != nil {
		return nil, fmt.Errorf("creating bin directory: %w", err)
	}

	outputBinary := binPath(projectRoot, outputName)
	karSource := filepath.Join(projectRoot, "src", "compiler", "main.kark")

	// Step a: Use the compiler to transpile main.kark -> C
	fmt.Printf("[Stage %d] Transpiling %s -> C using %s\n", stage, karSource, filepath.Base(compilerBinary))
	output, err := runCmdOutput(projectRoot, compilerBinary, "build", karSource, "--target", "c23")
	if err != nil {
		return nil, fmt.Errorf("karkain transpile failed: %v: %s", err, string(output))
	}
	_ = output

	// Step b: Find the generated C file
	cFile := findGeneratedCFile(projectRoot, karSource)
	if cFile == "" {
		return nil, fmt.Errorf("generated C file not found after transpilation")
	}
	// Always clean up the generated C file, even if gcc compilation fails, so a
	// stale artifact cannot shadow a freshly generated file in later stages/tests.
	defer os.Remove(cFile)

	// Step c: Compile with gcc
	fmt.Printf("[Stage %d] Compiling C -> %s\n", stage, outputBinary)
	if err := compileWithGCC(cFile, outputBinary); err != nil {
		return nil, fmt.Errorf("gcc compilation failed: %w", err)
	}

	sha, size, err := fileHash(outputBinary)
	if err != nil {
		return nil, err
	}

	result := &StageResult{
		Stage:    stage,
		Binary:   outputBinary,
		Size:     size,
		SHA256:   sha,
		Duration: time.Since(start),
	}
	fmt.Printf("[Stage %d] Done: %s (%d bytes, %s, %v)\n", stage, filepath.Base(outputBinary), size, sha[:16], result.Duration)
	return result, nil
}

// findGeneratedCFile looks for the C file generated by the karkain compiler
// next to the source .kark file. The compiler writes output with a .c23 extension.
func findGeneratedCFile(projectRoot, karSource string) string {
	// The compiler writes output with the target extension, e.g., .c23
	candidate := karSource[:len(karSource)-len(filepath.Ext(karSource))] + ".c23"
	if _, err := os.Stat(candidate); err == nil {
		return candidate
	}
	// Also check .c extension
	candidate = karSource[:len(karSource)-len(filepath.Ext(karSource))] + ".c"
	if _, err := os.Stat(candidate); err == nil {
		return candidate
	}
	return ""
}

// compileWithGCC compiles a self-contained C source file (with an embedded
// by-value runtime emitted by the codegen preamble) into an output binary.
// IMPORTANT: the legacy pointer-based src/compiler/runtime.c is deliberately
// NOT linked — the generated main.c (and compiler2/compiler3 outputs) already
// define every runtime helper by-value, and linking runtime.c causes duplicate
// symbol errors. The runtimeFile parameter was removed to make this explicit
// (see docs/audit/C-ABI.md).
func compileWithGCC(cFile, outputBinary string) error {
	args := []string{
		"-std=c2x",
		"-o", outputBinary,
		// The karkain compiler writes generated C with a target extension such
		// as ".c23", which gcc does not recognize as a C source by extension.
		// Force the input to be treated as C so gcc compiles it (instead of
		// handing an unknown file straight to the linker -> "file format not
		// recognized").
		"-x", "c",
		cFile,
		"-lm",
		"-lgmp",
	}

	return runCmd(filepath.Dir(outputBinary), "gcc", args...)
}
