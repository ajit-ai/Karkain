package cli

// Phase 123 — Language Core Completion: enum/ADT codegen + match parity.
//
// Phase 123 Part A completes the enum (algebraic data type) surface on the
// self-hosted kcc engine with byte-identical Go↔kcc behavior:
//
//   - enum declarations with unit and payload variants (`enum Color { Red,
//     Green, Blue }` / `enum Shape { Circle(r), Rect(w), Unit }`);
//   - `EnumName.Variant` construction in expression position, equality
//     comparison, and tag-only match-arm patterns (`Shape.Circle => ...`);
//   - check-time enum validation in the self-hosted checker (K106 for an
//     unknown enum type, K113 for an unknown variant), covering BOTH
//     expression construction and match-arm patterns;
//   - the same `expected '=>' in match arm` parse error as the Go parser when
//     a match-arm pattern is followed by anything other than `=>` (Go does not
//     support payload destructuring `Shape.Circle(r)` in patterns);
//   - parser-level enum-variant registration so `EnumName.Variant` parses as a
//     dedicated EnumVariantExpr node, not a generic member access;
//   - NODE_ENUM_VARIANT_EXPR does not collide with NODE_CONST_DECL (both being
//     94 was corrupted KIR rendering of `EnumName.Variant` expressions).
//
// The pinned golden examples (examples/01-fundamentals/13_enums.kark and
// 14_adt_match.kark) are byte-identical on both engines and are additionally
// pinned in the Phase 114 corpus gate; this suite focuses on the language-core
// fabric: the new kcc-only enum/match checks and the parse-error parity that
// the corpus goldens do not exercise.
//
// Deliberate strictness divergence (documented, NOT a defect): the kcc checker
// rejects unknown enum variants (K113) and undeclared enum types used in
// EnumName.Variant construction or match-arm patterns (K106) at CHECK time,
// while the Go resolver defers such validation to the C compiler (Go check
// passes, the build fails on an undeclared tag constant). kcc also reports an
// undeclared dotted base (`MissingKind.Purple`) as K102 at check time, matching
// Go's K002. Both engines ultimately reject all four negative fixtures.

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// phase123EnumFixtures maps the Phase 123 negative fixtures (created under
// examples/type_errors/) to the kcc check-time error code each must produce.
var phase123EnumFixtures = []struct {
	name string
	code string
	want string
}{
	{"err15_unknown_enum_variant_match", "K113", "Color.Purple"},
	{"err16_undefined_enum_match_arm", "K106", "Missing"},
	{"err17_undefined_enum_expr", "K102", "MissingKind"},
	{"err18_unknown_enum_variant_expr", "K113", "Color.Purple"},
}

// TestPhase123_EnumMatchChecker parses and type-checks every enum-positive and
// enum-negative fixture through the self-hosted checker, asserting the exact
// K1XX code and the offending name in the diagnostic.
func TestPhase123_EnumMatchChecker(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping self-hosted Phase 123 gate in short mode")
	}
	bin := phase95KCC(t)
	selectKCCEngine(t, bin)
	hasGCC(t)
	root := repoRoot(t)

	// Positive enum programs must pass check and never report a diagnostic.
	positives := []string{
		filepath.Join(root, "examples", "01-fundamentals", "13_enums.kark"),
		filepath.Join(root, "examples", "01-fundamentals", "14_adt_match.kark"),
	}
	for _, file := range positives {
		var buf bytes.Buffer
		res := KCCCheckCommand(&buf, file, false)
		if res.ExitCode != ExitSuccess {
			t.Errorf("kcc check rejected valid enum program %s (exit %d):\n%s",
				file, res.ExitCode, strings.TrimSpace(res.Message))
		} else if !strings.Contains(res.Message, "[ok]") {
			t.Errorf("kcc check of %s succeeded without the [ok] marker", file)
		}
	}

	// Negative fixtures must exit 3, mention the expected code + name, and
	// never be reported as [ok].
	for _, fx := range phase123EnumFixtures {
		file := filepath.Join(root, "examples", "type_errors", fx.name, "main.kark")
		var buf bytes.Buffer
		res := KCCCheckCommand(&buf, file, false)
		out := res.Message
		if res.ExitCode != ExitCompile {
			t.Errorf("fixture %s: want ExitCompile, got %d:\n%s", fx.name, res.ExitCode, strings.TrimSpace(out))
		}
		if strings.Contains(out, "[ok]") {
			t.Errorf("fixture %s: rejected program still reported as [ok]", fx.name)
		}
		if !strings.Contains(out, fx.code) {
			t.Errorf("fixture %s: output lacks expected error code %s:\n%s", fx.name, fx.code, strings.TrimSpace(out))
		}
		if !strings.Contains(out, fx.want) {
			t.Errorf("fixture %s: output lacks expected offending name %q:\n%s", fx.name, fx.want, strings.TrimSpace(out))
		}
	}
}

// TestPhase123_MatchArmParseErrorParity verifies the `=>` guard added to the
// kcc parseMatchExpr mirrors the Go parser exactly: a match-arm pattern
// followed by anything other than `=>` (here, payload destructuring — not part
// of the language) is rejected with the identical diagnostic on both engines.
func TestPhase123_MatchArmParseErrorParity(t *testing.T) {
	src := `enum Shape { Circle(r), Rect(w), Unit }

func area(s) {
    return match s {
        Shape.Circle(r) => 10,
        Shape.Rect => 20,
        _ => 30,
    }
}

func main() {
    let s = Shape.Circle(5)
    print(area(s))
}
`
	dir := t.TempDir()
	path := filepath.Join(dir, "main.kark")
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	const want = "expected '=>' in match arm, got '('"

	// Both engines route syntax errors through the Go-side Phase 105 preflight
	// (projectSyntaxDiagnostics), which renders to stderr — so drive the real
	// binary with CombinedOutput, exactly like `karkain check`, for each leg.
	bin := buildPreviewBinary(t)
	root := repoRoot(t)

	runEngine := func(engine string) (string, error) {
		cmd := exec.Command(bin, "check", path)
		cmd.Dir = root
		cmd.Env = append(os.Environ(), "KARKAIN_ENGINE="+engine)
		out, err := cmd.CombinedOutput()
		return string(out), err
	}

	for _, engine := range []string{"go", "kcc"} {
		out, err := runEngine(engine)
		if err == nil {
			t.Fatalf("%s engine check should fail for destructuring pattern", engine)
		}
		if !strings.Contains(out, want) {
			t.Fatalf("%s engine output lacks %q:\n%s", engine, want, out)
		}
	}
}

// TestPhase123_EnumVariantExpressionKIR verifies the EnumVariantExpr node
// renders correctly in KIR after the NODE_ENUM_VARIANT_EXPR=94/95 collision
// fix: construction must render as `(enum_variant EnumName Variant ...)`, not
// as a corrupted `(expr <ConstDecl>)` node.
func TestPhase123_EnumVariantExpressionKIR(t *testing.T) {
	bin := buildPreviewBinary(t)
	root := repoRoot(t)
	dir := t.TempDir()
	fixture := filepath.Join(dir, "main.kark")
	src := `enum Color { Red, Green, Blue }

func main() {
    let a = Color.Red
    let c = Color.Blue
    print(a)
    print(c)
}
`
	if err := os.WriteFile(fixture, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := runBin(t, bin, root, "kir", fixture)
	if err != nil {
		t.Fatalf("karkain kir failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, "enum Color (Red:") {
		t.Errorf("KIR missing enum declaration header:\n%s", out)
	}
	if !strings.Contains(out, "let a = (enum_variant Color Red)") {
		t.Errorf("KIR missing EnumVariantExpr construction for Color.Red:\n%s", out)
	}
	if !strings.Contains(out, "let c = (enum_variant Color Blue)") {
		t.Errorf("KIR missing EnumVariantExpr construction for Color.Blue:\n%s", out)
	}
	if strings.Contains(out, "<ConstDecl>") {
		t.Errorf("KIR corrupted EnumVariantExpr as const-decl reference:\n%s", out)
	}
}