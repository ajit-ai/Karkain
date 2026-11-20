package sema

import (
	"strings"
	"testing"

	"karkain/pkg/lexer"
	"karkain/pkg/parser"
)

func typeFirstProgram(t *testing.T, src string) []ResolveError {
	t.Helper()
	p := parser.New(lexer.New(src))
	prog := parser.ApplyMacroExpansion(p.ParseProgram())
	if len(p.Errors) > 0 {
		t.Fatalf("unexpected parse errors: %v", p.Errors)
	}
	return NewResolver(prog, nil).Resolve()
}

func TestTypeFirstPositive(t *testing.T) {
	src := `
int age = 54;
double salary = 85000.50;
string name = "Karkain";
bool active = true;
char grade = 'A';
int[] values = [10, 20, 30];
int[][] matrix = [[1, 2], [3, 4]];
const int MAX_SIZE = 100;
int add(int a, int b) { return a + b; }
func main() {
    int result = add(10, 20);
    print(result);
    print(matrix[1][0]);
    print(MAX_SIZE);
}
`
	if errs := typeFirstProgram(t, src); len(errs) > 0 {
		t.Fatalf("valid type-first program rejected: %v", errs)
	}
}

func TestTypeFirstValueNominal(t *testing.T) {
	src := `
Value makeNode(int t) {
    Value n = []
    return n
}
func main() {
    Value x = makeNode(1)
    print(x)
}
`
	if errs := typeFirstProgram(t, src); len(errs) > 0 {
		t.Fatalf("valid nominal Value program rejected: %v", errs)
	}
}

func TestTypeFirstNegative(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want string
	}{
		{"bad initializer", `func main() { int x = "text"; print(x); }`, "cannot initialize"},
		{"bad argument", `int add(int a, int b) { return a + b; }
func main() { int x = add(1, "two"); print(x); }`, "argument 2"},
		{"bad return", `int wrong() { return "text"; }
func main() { print(1); }`, "return type"},
		{"bad array element", `func main() { int[] a = [1, "x"]; print(a); }`, "array element type"},
		{"byte overflow", `func main() { byte b = 300; print(b); }`, "outside the range"},
		{"value mismatch", `func main() { Value x = 5; print(x); }`, "cannot initialize"},
		{"map value mismatch", `func main() { map[string]int m = {"a": "wrong"}; print(m); }`, "map value type"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			errs := typeFirstProgram(t, tc.src)
			for _, e := range errs {
				if strings.Contains(e.Msg, tc.want) {
					return
				}
			}
			t.Fatalf("expected diagnostic containing %q, got %v", tc.want, errs)
		})
	}
}
