package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"karkain/pkg/codegen"
	"karkain/pkg/lexer"
	"karkain/pkg/parser"
)

// runKarkFile compiles+runs an existing .kark file and returns stdout.
// The source is copied to a temp dir first so the codegen never writes
// sibling .c artifacts next to the original file.
func runKarkFile(t *testing.T, path string) string {
	t.Helper()
	hasGCC(t)
	work := t.TempDir()
	copyPath := filepath.Join(work, filepath.Base(path))
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if err := os.WriteFile(copyPath, src, 0o644); err != nil {
		t.Fatalf("copy %s: %v", path, err)
	}
	return compileRunSource(t, string(src), copyPath, work)
}

// compileRunSource runs the full compile+run cycle for a source text, retrying
// transient gcc failures (Windows toolchain file locks under parallel load).
func compileRunSource(t *testing.T, src, refPath, work string) string {
	t.Helper()
	const attempts = 4
	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		cfg := codegen.NewConfig()
		cfg.CompileOnly = false
		cfg.RunAfter = false
		cfg.OutputPath = filepath.Join(work, fmt.Sprintf("prog_%d", attempt))

		l := lexer.New(src)
		p := parser.New(l)
		prog := p.ParseProgram()
		if len(p.Errors) > 0 {
			t.Fatalf("parse errors: %s", strings.Join(p.Errors, "; "))
		}
		prog = parser.ApplyMacroExpansion(prog)
		g := codegen.New(cfg)
		if err := g.GenerateAndCompile(prog, refPath); err != nil {
			lastErr = err
			if strings.Contains(err.Error(), "C compilation failed") && attempt < attempts {
				time.Sleep(500 * time.Millisecond)
				continue
			}
			t.Fatalf("compile failed: %v", err)
		}
		exe := cfg.OutputPath
		if runtime.GOOS == "windows" {
			exe += ".exe"
		}
		out, err := exec.Command(exe).CombinedOutput()
		if err != nil {
			if attempt < attempts {
				lastErr = err
				time.Sleep(500 * time.Millisecond)
				continue
			}
			t.Fatalf("run failed: %v output=%s", err, string(out))
		}
		return string(out)
	}
	t.Fatalf("transient failures running compiled unit: %v", lastErr)
	return ""
}

// expectedAlgorithmOutput maps each algorithm program name to its exact
// expected stdout (verified against reference results).
var expectedAlgorithmOutput = map[string]string{
	"bubble":      "Bubble sort result:\n11\n12\n22\n25\n34\n64\n90",
	"quick":       "QuickSort result:\n1\n5\n7\n8\n9\n10",
	"merge":       "MergeSort result:\n3\n9\n10\n27\n38\n43\n82",
	"heap":        "HeapSort result:\n5\n6\n7\n11\n12\n13",
	"linear":      "Linear Search for 60:\n5\nLinear Search for 65 (not found):\n-1",
	"binary":      "Binary Search for 40:\n3\nBinary Search for 45 (not found):\n-1",
	"ternary":     "Ternary Search for 80:\n7\nTernary Search for 75 (not found):\n-1",
	"kadane":      "Kadane's max subarray sum:\n6",
	"knapsack":    "0/1 Knapsack max value:\n9",
	"levenshtein": "Levenshtein distance 'kitten' -> 'sitting':\n3\nLevenshtein distance 'sunday' -> 'saturday':\n3",
	"bfs":         "BFS: start 0 reachable to 5?\n1\nBFS: start 0 reachable to 4?\n1",
	"dfs":         "DFS: start 0 visits 5?\n1\nDFS: start 0 visits 3?\n1",
	"dijkstra":    "Dijkstra shortest 0 -> 3:\n5",
	"bellman":     "Bellman-Ford shortest 0 -> 3:\n5",
	"factorial":   "Factorial of 5:\n120\nFactorial of 10:\n3628800",
	"fibonacci":   "Fibonacci(10):\n55\nFibonacci(20):\n6765",
	"gcd":         "GCD(48, 36):\n12\nGCD(270, 192):\n6",
	"lcm":         "LCM(4, 6):\n12\nLCM(21, 6):\n42",
	"sieve":       "Primes up to 30:\n2\n3\n5\n7\n11\n13\n17\n19\n23\n29",
	"power":       "2^10:\n1024\n3^4:\n81",
	"absolute_value": "abs(-42):\n42\nabs(7):\n7\nabs(0):\n0",
}

// TestAlgorithmCorpus_RunsEveryProgram compiles and runs every program under
// examples/algorithms/*/main.kark and asserts its exact expected output. This
// is the regression net for the algorithm capability milestone: any change to
// the front-end, SSA IR emit, or C runtime that breaks a classic algorithm is
// caught here without golden-file drift.
func TestAlgorithmCorpus_RunsEveryProgram(t *testing.T) {
	root := filepath.Join("..", "..", "examples", "algorithms")
	if _, err := os.Stat(root); err != nil {
		t.Skipf("algorithm examples not present: %v", err)
	}
	dirs, err := os.ReadDir(root)
	if err != nil {
		t.Fatalf("read %s: %v", root, err)
	}
	ran := 0
	for _, d := range dirs {
		if !d.IsDir() {
			continue
		}
		name := d.Name()
		want, ok := expectedAlgorithmOutput[name]
		if !ok {
			t.Errorf("corpus lacks expectation for %s", name)
			continue
		}
		path := filepath.Join(root, name, "main.kark")
		if _, err := os.Stat(path); err != nil {
			t.Errorf("missing %s: %v", path, err)
			continue
		}
		got := strings.ReplaceAll(strings.TrimSpace(runKarkFile(t, path)), "\r\n", "\n")
		ran++
		if got != strings.TrimSpace(want) {
			t.Errorf("%s:\n  want: %q\n   got: %q", name, want, got)
		}
	}
	if ran == 0 {
		t.Fatal("no algorithm programs ran")
	}
}

// TestAlgorithmCorpus_EveryAlgorithmCovered guards against accidental deletion
// of either a program or its expectation: coverage must be complete.
func TestAlgorithmCorpus_EveryAlgorithmCovered(t *testing.T) {
	want := []string{
		"absolute_value", "bellman", "bfs", "binary", "bubble", "dfs", "dijkstra",
		"factorial", "fibonacci", "gcd", "heap", "kadane", "knapsack", "lcm",
		"levenshtein", "linear", "merge", "power", "quick", "sieve", "ternary",
	}
	if len(expectedAlgorithmOutput) != len(want) {
		t.Errorf("expected %d expectations, got %d", len(want), len(expectedAlgorithmOutput))
	}
	for _, name := range want {
		if _, ok := expectedAlgorithmOutput[name]; !ok {
			t.Errorf("missing expectation for %s", name)
		}
	}
}

// runKarkSource writes an inline .kark source to an isolated temp dir (no
// sibling-join contamination) and compiles+runs it, returning stdout.
func runKarkSource(t *testing.T, name, src string) string {
	t.Helper()
	dir := gccTempDir(t)
	path := writeTestFile(t, dir, name, src)
	return runKarkFile(t, path)
}

// basicConstructSources are regression probes for fundamental language
// constructs: recursion, strings + char indexing, string concat, nested loops,
// 2D arrays rebuild + write, maps via index syntax, integer/float modulo,
// struct field access, param mutation through arrays, slices, else-if chains,
// float arithmetic, and boolean shape (&&/||) + literal printing.
var basicConstructSources = map[string]string{
	"recursion": `func fib(n) {
    if (n <= 1) {
        return n
    }
    return fib(n - 1) + fib(n - 2)
}
func main() {
    print(fib(10))
}
`,
	"strings": `func main() {
    let s = "hello world"
    print(len(s))
    print(s[0])
    print(s[6])
}
`,
	"strings_concat": `func main() {
    let a = "foo"
    let b = "bar"
    print(a + b)
    print(a + b + a)
}
`,
	"nested_loops": `func main() {
    let total = 0
    let i = 0
    while (i < 3) {
        let j = 0
        while (j < 4) {
            total = total + 1
            j = j + 1
        }
        i = i + 1
    }
    print(total)
}
`,
	"array_2d": `func main() {
    let grid = []
    let r = 0
    while (r < 3) {
        let row = []
        let c = 0
        while (c < 3) {
            push(row, r * 3 + c)
            c = c + 1
        }
        push(grid, row)
        r = r + 1
    }
    print(grid[1][2])
    grid[2][1] = 99
    print(grid[2][1])
}
`,
	"maps": `func main() {
    let m = {}
    m["age"] = 30
    print(m["age"])
    m["name"] = "Ana"
    print(m["name"])
}
`,
	"modulo": `func main() {
    print(17 / 5)
    print(17 % 5)
    print(-7 % 3)
}
`,
	"structs": `type Person struct { name(string), age(int) }
func main() {
    let p = Person{name: "Ana", age: 25}
    print(p.name)
    print(p.age)
}
`,
	"param_mutation": `func bump(arr, idx) {
    arr[idx] = arr[idx] + 10
}
func main() {
    let xs = [1, 2, 3]
    bump(xs, 1)
    print(xs[1])
}
`,
	"slices": `func main() {
    let xs = [10, 20, 30, 40, 50]
    let tail = xs[2:5]
    print(len(tail))
    print(tail[0])
    print(tail[2])
}
`,
	"guard": `func classify(n) {
    if (n < 0) {
        return -1
    } else if (n == 0) {
        return 0
    } else if (n < 100) {
        return 1
    }
    return 2
}
func main() {
    print(classify(-5))
    print(classify(0))
    print(classify(50))
    print(classify(500))
}
`,
	"floats": `func main() {
    print(7.0 / 2.0)
    print(3.5 + 1.25)
    print(2.5 * 4.0)
    print(5.0 % 2.0)
}
`,
	"logic": `func main() {
    let x = 5
    if (x > 0 && x < 10) {
        print(1)
    }
    if (x > 10 || x == 5) {
        print(2)
    }
}
`,
	"bool_print": `func main() {
    print(true)
    print(false)
}
`,
}

var basicConstructExpected = map[string]string{
	"recursion":      "55",
	"strings":        "11\nh\nw",
	"strings_concat": "foobar\nfoobarfoo",
	"nested_loops":   "12",
	"array_2d":       "5\n99",
	"maps":           "30\nAna",
	"modulo":         "3\n2\n-1",
	"structs":        "Ana\n25",
	"param_mutation": "12",
	"slices":         "3\n30\n50",
	"guard":          "-1\n0\n1\n2",
	"floats":         "3.5\n4.75\n10\n1",
	"logic":          "1\n2",
	"bool_print":     "true\nfalse",
}

// TestBasicConstructsCorpus compiles+runs inline probes of fundamental
// constructs and asserts exact outputs. This is the "basics" safety net: any
// change to parsing, semantics, or codegen that breaks a core construct fails
// here before modules/periphery are touched.
func TestBasicConstructsCorpus(t *testing.T) {
	checked := 0
	for name, src := range basicConstructSources {
		want, ok := basicConstructExpected[name]
		if !ok {
			t.Errorf("basic corpus %s lacks expected output", name)
			continue
		}
		got := strings.ReplaceAll(strings.TrimSpace(runKarkSource(t, name+".kark", src)), "\r\n", "\n")
		checked++
		if got != want {
			t.Errorf("%s:\n  want: %q\n   got: %q", name, want, got)
		}
	}
	if checked == 0 {
		t.Fatal("no basic-construct programs ran")
	}
}