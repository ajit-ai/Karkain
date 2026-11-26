// v1.2.20 - JSON full DOM on both engines (C23 path).
//
// The gate exercises a real recursive-descent DOM: parse, type check, nested
// path query, escaping/serialization round-trip and deterministic error
// strings. Evidence is local string-only; native/WASM remain excluded.
package cli

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

var v1220Once sync.Once
var v1220Bin string
var v1220Err error

func v1220Prepare(t *testing.T) string {
	t.Helper()
	v1220Once.Do(func() {
		for _, dir := range []string{
			`C:\Program Files\Git\cmd`,
			`C:\Windows\System32\WindowsPowerShell\v1.0`,
			`C:\msys64\mingw64\bin`,
		} {
			if st, err := os.Stat(dir); err == nil && st.IsDir() {
				parts := strings.Split(os.Getenv("PATH"), string(os.PathListSeparator))
				found := false
				for _, p := range parts {
					if strings.EqualFold(p, dir) {
						found = true
						break
					}
				}
				if !found {
					_ = os.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
				}
			}
		}
		v1220Err = buildKarkainErr(t, &v1220Bin)
	})
	if v1220Err != nil {
		t.Fatalf("building karkain: %v", v1220Err)
	}
	return v1220Bin
}

func v1220Source(t *testing.T, name, body string) string {
	t.Helper()
	dir, err := os.MkdirTemp(repoRoot(t), "v1220")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	src := "import std.json\n" + body
	path := filepath.Join(dir, name+".kark")
	if err := os.WriteFile(path, []byte(src), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func v1220Run(t *testing.T, engine, path string) string {
	t.Helper()
	karkain := v1220Prepare(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, karkain, "run", path, "--engine", engine)
	// Compiler warnings must not be parsed as program output on Linux CI.
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if ctx.Err() != nil {
		t.Fatalf("%s timed out: %v\nstdout:\n%s\nstderr:\n%s", engine, ctx.Err(), stdout.String(), stderr.String())
	}
	if err != nil {
		t.Fatalf("%s failed: %v\nstdout:\n%s\nstderr:\n%s", engine, err, stdout.String(), stderr.String())
	}
	return stdout.String()
}

const v1220DOMProgram = `func main() {
    let text = "{\"name\":\"kai\",\"n\":1,\"pi\":1.5,\"ok\":true,\"nothing\":null,\"tags\":[\"a\",\"b\"],\"nested\":{\"x\":2}}"
    let doc = json_dom_parse(text)
    print(json_dom_type(doc))
    print(json_dom_get(doc, "name"))
    print(json_dom_get(doc, "n"))
    print(json_dom_get(doc, "pi"))
    print(json_dom_get(doc, "ok"))
    print(json_dom_get(doc, "nothing"))
    print(json_dom_type(json_dom_get(doc, "nothing")))
    print(json_dom_get(doc, "tags[1]"))
    print(json_dom_get(doc, "nested.x"))
    print(json_dom_serialize(doc))
    print(json_dom_error())
}
`

// TestPhaseV1220_DOMBothEngines proves nested parse/query/serialize behaviour
// and raw stdout parity between Go and kcc. Exact values are local and fixed.
func TestPhaseV1220_DOMBothEngines(t *testing.T) {
	path := v1220Source(t, "dom", v1220DOMProgram)
	var first string
	for i, engine := range []string{"go", "kcc"} {
		t.Run(engine, func(t *testing.T) {
			out := v1220Run(t, engine, path)
			if i == 0 {
				first = out
			} else if out != first {
				t.Errorf("engine stdout differs:\ngo:\n%s\n%s:\n%s", first, engine, out)
			}
			lines := strings.Split(strings.ReplaceAll(out, "\r\n", "\n"), "\n")
			want := []string{
				"object", "kai", "1", "1.5", "true", "__json_null__", "null",
				"b", "2",
				`{"name":"kai","n":1,"pi":1.5,"ok":true,"nothing":null,"tags":["a","b"],"nested":{"x":2}}`,
			}
			if len(lines) < len(want) {
				t.Fatalf("got %d lines, want at least %d:\n%s", len(lines), len(want), out)
			}
			for j, w := range want {
				if strings.TrimSpace(lines[j]) != w {
					t.Errorf("line %d = %q, want %q", j, strings.TrimSpace(lines[j]), w)
				}
			}
			if strings.TrimSpace(lines[len(want)]) != "" {
				t.Errorf("json_dom_error() = %q, want empty", strings.TrimSpace(lines[len(want)]))
			}
		})
	}
}

const v1220EscapeProgram = `func main() {
    let text = "{\"s\":\"a\\\"b\\\\c\\nd\",\"u\":\"\\u00e9\"}"
    let doc = json_dom_parse(text)
    print(json_dom_get(doc, "s"))
    print(json_dom_get(doc, "u"))
    print(json_dom_serialize(doc))
    print(json_dom_error())
}
`

// TestPhaseV1220_EscapesRoundTrip pins escape decoding and re-encoding.
func TestPhaseV1220_EscapesRoundTrip(t *testing.T) {
	path := v1220Source(t, "escape", v1220EscapeProgram)
	var first string
	for i, engine := range []string{"go", "kcc"} {
		t.Run(engine, func(t *testing.T) {
			out := v1220Run(t, engine, path)
			if i == 0 {
				first = out
			} else if out != first {
				t.Errorf("engine stdout differs:\ngo:\n%s\n%s:\n%s", first, engine, out)
			}
			if !strings.Contains(out, "a\"b\\c") {
				t.Errorf("escaped string not decoded:\n%s", out)
			}
			if !strings.Contains(out, "é") {
				t.Errorf("\\u00e9 not decoded:\n%s", out)
			}
			if !strings.Contains(out, `\"`) || !strings.Contains(out, `\\`) || !strings.Contains(out, `\n`) {
				t.Errorf("serialize did not re-escape correctly:\n%s", out)
			}
		})
	}
}

const v1220ErrorProgram = `func main() {
    let bad = json_dom_parse("{\"a\":}")
    print(len(bad))
    print(json_dom_error())
    let trail = json_dom_parse("{} x")
    print(len(trail))
    print(json_dom_error())
}
`

// TestPhaseV1220_DeterministicErrors pins fixed, non-empty failure strings.
func TestPhaseV1220_DeterministicErrors(t *testing.T) {
	path := v1220Source(t, "errors", v1220ErrorProgram)
	var first string
	for i, engine := range []string{"go", "kcc"} {
		t.Run(engine, func(t *testing.T) {
			out := v1220Run(t, engine, path)
			if i == 0 {
				first = out
			} else if out != first {
				t.Errorf("engine error text differs:\ngo:\n%s\n%s:\n%s", first, engine, out)
			}
			lines := strings.Split(strings.ReplaceAll(out, "\r\n", "\n"), "\n")
			for idx := 0; idx < 4; idx += 2 {
				if strings.TrimSpace(lines[idx]) != "0" {
					t.Errorf("parse failure produced a non-empty value at line %d:\n%s", idx, out)
				}
				if !strings.HasPrefix(strings.TrimSpace(lines[idx+1]), "json:") {
					t.Errorf("missing deterministic json error at line %d:\n%s", idx+1, out)
				}
			}
		})
	}
}

// TestPhaseV1220_RuntimeWiring proves the runtime is emitted only for JSON
// programs. Importing std.json is enough because its wrappers call the builtins.
func TestPhaseV1220_RuntimeWiring(t *testing.T) {
	karkain := v1220Prepare(t)
	dir, err := os.MkdirTemp(repoRoot(t), "v1220wire")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	cases := map[string]string{
		"json":  "import std.json\n" + v1220DOMProgram,
		"plain": "func main() { print(\"plain\") }\n",
	}
	for name, src := range cases {
		path := filepath.Join(dir, name+".kark")
		out := filepath.Join(dir, name+".c")
		if err := os.WriteFile(path, []byte(src), 0644); err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		cmd := exec.CommandContext(ctx, karkain, "build", path, "--engine", "go", "--target", "c23", "-o", out)
		if msg, err := cmd.CombinedOutput(); err != nil {
			cancel()
			t.Fatalf("%s c23 build failed: %v\n%s", name, err, string(msg))
		}
		cancel()
		data, err := os.ReadFile(out)
		if err != nil {
			t.Fatal(err)
		}
		hasRuntime := strings.Contains(string(data), "karkain_json_parse(")
		if name == "json" && !hasRuntime {
			t.Errorf("JSON program did not emit the DOM runtime")
		}
		if name == "plain" && hasRuntime {
			t.Errorf("JSON-free program emitted the DOM runtime")
		}
	}
}

// TestPhaseV1220_NativeRefused pins the platform boundary.
func TestPhaseV1220_NativeRefused(t *testing.T) {
	karkain := v1220Prepare(t)
	// No import: importing std.json pulls the module wrappers, whose string
	// operators are refused earlier by the limited native driver. The platform
	// boundary under test is the builtin itself.
	dir, err := os.MkdirTemp(repoRoot(t), "v1220native")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	path := filepath.Join(dir, "native.kark")
	src := "func main() { let v = json_parse(\"{}\") }\n"
	if err := os.WriteFile(path, []byte(src), 0644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "native.exe")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, karkain, "build", path, "--engine", "go",
		"--target", "native-x86_64-windows", "-o", out)
	msg, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("native build unexpectedly succeeded:\n%s", string(msg))
	}
	if !strings.Contains(string(msg), "json_") {
		t.Errorf("refusal must name a json_* builtin, got:\n%s", string(msg))
	}
	if _, statErr := os.Stat(out); statErr == nil {
		t.Errorf("native refusal should not write an image")
	}
}
