// Phase v1.2.15 - macOS execution (native image executes on macos-13 runner).
//
// Mirrors TestNativePE for the Mach-O container: the same goldens, executed
// where they can run (darwin/amd64 - the macos-13 CI leg). Elsewhere the
// test skips honestly via runNativeMacOS. This is the execution evidence the
// structural TestNativeMachO pins could not provide.
package native

import (
	"testing"
)

func TestNativeMachOExec(t *testing.T) {
	cases := []struct {
		name     string
		src      string
		want     string
		wantCode int
	}{
		{"empty", "func main() {\n}\n", "", 0},
		{"retcode", "func main() {\n    return 7\n}\n", "", 7},
		{"int42", "func main() {\n    print(42)\n}\n", "42\n", 0},
		{"str", "func main() {\n    print(\"hi\")\n}\n", "hi\n", 0},
		{"add", "func add(a, b) {\n    return a + b\n}\nfunc main() {\n    print(add(20, 22))\n}\n", "42\n", 0},
		{"while_sum", "func main() {\n    let s = 0\n    let i = 1\n    while (i <= 10) {\n        s = s + i\n        i = i + 1\n    }\n    print(s)\n}\n", "55\n", 0},
		{"string_arg", "func greet(name string) {\n    print(name)\n}\nfunc main() {\n    greet(\"yo\")\n}\n", "yo\n", 0},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			img := compileNativeOS(t, OSMacOS, c.src)
			out, code := runNativeMacOS(t, img)
			requireNativeOut(t, c.name, out, c.want, code, c.wantCode)
		})
	}
}