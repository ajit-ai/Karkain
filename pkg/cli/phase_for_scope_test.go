// Unnumbered corrective slice on the v1.2.x line: C-style `for` scope leak in
// the self-hosted checker.
//
// Measured at the committed baseline: Go `check` rejects a binding introduced
// by a C-style `for` that is read after the loop (`error[K002]`) because
// pkg/sema/resolve.go's walkLocalDefs has no ForStmt case. kcc accepted the
// same program ([ok], exit 0): collectLocals seeded for-init and for-body names
// into the FUNCTION's permanent locals table and pass 2 never removed them.
//
// This is a differential, not a kcc golden. A golden written against kcc's
// baseline bytes would have kept passing for the whole time the defect was
// present -- the exact reason the Go engine is asserted here as the reference.
//
// The accepted half is just as load-bearing: a forward reference inside the
// body (`print(y); let y = 5`) is accepted by BOTH engines, so the fix must
// keep body pre-seeding and only end the scope at the closing brace.
package cli

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

var forScopeOnce sync.Once
var forScopeCLI, forScopeKCC string
var forScopeErr error

func forScopePrepare(t *testing.T) (string, string) {
	t.Helper()
	forScopeOnce.Do(func() {
		// gcc / GMP are not on the default PATH on this host; CI's Windows leg
		// needs the same prelude. Mirror v1226Prepare rather than depend on it.
		for _, dir := range []string{
			`C:\Program Files\Git\cmd`,
			`C:\Windows\System32\WindowsPowerShell\v1.0`,
			`C:\msys64\mingw64\bin`,
			`C:\msys64\ucrt64\bin`,
		} {
			if st, err := os.Stat(dir); err != nil || !st.IsDir() {
				continue
			}
			found := false
			for _, p := range strings.Split(os.Getenv("PATH"), string(os.PathListSeparator)) {
				if strings.EqualFold(p, dir) {
					found = true
					break
				}
			}
			if !found {
				_ = os.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
			}
		}

		var cli string
		if forScopeErr = buildKarkainErr(t, &cli); forScopeErr != nil {
			return
		}

		// Build kcc from the WORKING TREE, never from an inherited
		// KARKAIN_KCC: a cached binary built before the checker fix would
		// make the kcc leg fail for an environmental reason and prove nothing.
		old, had := os.LookupEnv("KARKAIN_KCC")
		_ = os.Unsetenv("KARKAIN_KCC")
		built := phase95KCC(t)
		if had {
			_ = os.Setenv("KARKAIN_KCC", old)
		} else {
			_ = os.Unsetenv("KARKAIN_KCC")
		}
		// phase95KCC's sandbox is owned by the calling test; copy the binary
		// out so later subtests do not see "KARKAIN_KCC points to missing
		// binary" (the p02KCC lesson).
		dir, err := os.MkdirTemp("", "karkain-for-scope-kcc")
		if err != nil {
			forScopeErr = err
			return
		}
		data, err := os.ReadFile(built)
		if err != nil {
			forScopeErr = err
			return
		}
		dst := filepath.Join(dir, "kcc.exe")
		if err := os.WriteFile(dst, data, 0o755); err != nil {
			forScopeErr = err
			return
		}
		forScopeCLI, forScopeKCC, forScopeErr = cli, dst, nil
	})
	if forScopeErr != nil {
		t.Fatalf("preparing for-scope gate: %v", forScopeErr)
	}
	return forScopeCLI, forScopeKCC
}

// forScopeSource gives each case its OWN directory. assembleProject adopts
// every sibling .kark that does not declare a main, so a shared directory
// would silently mix cases (the Step 9d scratch-directory trap).
func forScopeSource(t *testing.T, name, body string) string {
	t.Helper()
	dir, err := os.MkdirTemp(repoRoot(t), "forscope")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	path := filepath.Join(dir, name+".kark")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// forScopeInvoke drives the real binary. stdout and stderr are kept separate:
// the Go engine streams the child with inherited stdio while the kcc engine
// captures it, and the kcc build banner must not be mistaken for a diagnostic.
func forScopeInvoke(t *testing.T, engine, command, path string, retryCompile bool, extra ...string) (string, string, int) {
	t.Helper()
	cli, kcc := forScopePrepare(t)
	for attempt := 0; attempt < 2; attempt++ {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		args := append([]string{command, path, "--engine", engine}, extra...)
		cmd := exec.CommandContext(ctx, cli, args...)
		if engine == "kcc" {
			cmd.Env = append(os.Environ(), "KARKAIN_KCC="+kcc)
		}
		var out, errOut bytes.Buffer
		cmd.Stdout = &out
		cmd.Stderr = &errOut
		err := cmd.Run()
		timedOut := ctx.Err() != nil
		cancel()
		if timedOut {
			t.Fatalf("%s %s timed out\nstdout:\n%s\nstderr:\n%s", engine, command, out.String(), errOut.String())
		}
		code := 0
		if err != nil {
			if ee, ok := err.(*exec.ExitError); ok {
				code = ee.ExitCode()
			} else {
				t.Fatalf("%s %s: %v\nstdout:\n%s\nstderr:\n%s", engine, command, err, out.String(), errOut.String())
			}
		}
		// This host intermittently KILLS gcc while the pagefile thrashes: the
		// C backend reports "C compilation failed: exit status 1" with no
		// compiler diagnostic. Retry a COMPILATION failure only -- a program
		// failure is deterministic and must never be retried away.
		if retryCompile && attempt == 0 && code != 0 &&
			(strings.Contains(out.String(), "C compilation failed") || strings.Contains(errOut.String(), "C compilation failed")) {
			continue
		}
		return out.String(), errOut.String(), code
	}
	panic("unreachable")
}

const forScopeLetInitLeak = `func main() {
    for (let i = 0; i < 2; i = i + 1) {
        print(i)
    }
    print(i)
}
`

const forScopeVarInitLeak = `func main() {
    for (var i = 0; i < 2; i = i + 1) {
        print(i)
    }
    print(i)
}
`

const forScopeBodyLetLeak = `func main() {
    for (let i = 0; i < 2; i = i + 1) {
        let y = 5
        print(y)
    }
    print(y)
}
`

const forScopeAssignLeak = `func main() {
    for (let i = 0; i < 2; i = i + 1) {
        z = 7
    }
    print(z)
}
`

const forScopeForward = `func main() {
    for (let i = 0; i < 1; i = i + 1) {
        print(y)
        let y = 5
    }
}
`

const forScopeForInAfter = `func main() {
    let a = [1, 2, 3]
    for x in a {
        print(x)
    }
    print(x)
}
`

const forScopeWhileAfter = `func main() {
    let n = 0
    while (n < 1) {
        let w = 3
        n = n + 1
    }
    print(w)
}
`

func TestPhaseForScope_LeakedBindingsRejectedByBothEngines(t *testing.T) {
	cases := []struct {
		name, src, ident string
	}{
		{"a_letinitleak", forScopeLetInitLeak, "i"},
		{"f_forvar", forScopeVarInitLeak, "i"},
		{"i_bodyleak", forScopeBodyLetLeak, "y"},
		{"k_assignleak", forScopeAssignLeak, "z"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := forScopeSource(t, tc.name, tc.src)

			// Go is the reference engine for this contract. Assert the CODE
			// and the NAME, not merely a non-zero status: an unrelated parse
			// failure would also exit 3 and must not satisfy the gate.
			goOut, goErr, goCode := forScopeInvoke(t, "go", "check", path, false)
			goAll := goOut + goErr
			if goCode != 3 {
				t.Errorf("go check exit = %d, want 3\nstdout:\n%s\nstderr:\n%s", goCode, goOut, goErr)
			}
			if !strings.Contains(goAll, "error[K002]") {
				t.Errorf("go check did not report K002\n%s", goAll)
			}
			if !strings.Contains(goAll, "undefined identifier '"+tc.ident+"'") {
				t.Errorf("go check did not name %q\n%s", tc.ident, goAll)
			}

			// kcc must agree, through its OWN checker: K102 is emitted only
			// from checkIdent, so this cannot be satisfied by the Go preflight.
			kccOut, kccErr, kccCode := forScopeInvoke(t, "kcc", "check", path, false)
			kccAll := kccOut + kccErr
			if kccCode != 3 {
				t.Errorf("kcc check exit = %d, want 3\nstdout:\n%s\nstderr:\n%s", kccCode, kccOut, kccErr)
			}
			if !strings.Contains(kccAll, "error[K102]") {
				t.Errorf("kcc check did not report K102\n%s", kccAll)
			}
			if !strings.Contains(kccAll, "undefined identifier '"+tc.ident+"'") {
				t.Errorf("kcc check did not name %q\n%s", tc.ident, kccAll)
			}
			if strings.Contains(kccAll, "[ok]") {
				t.Errorf("kcc accepted a leaking binding\n%s", kccAll)
			}
		})
	}
}

func TestPhaseForScope_AcceptedProgramsKeepTheirMeaning(t *testing.T) {
	// check-only boundary pins. These two shapes are ACCEPTED by both engines
	// today; the for fix must not spread to for-in, while or Block scoping.
	// They are deliberately not run (the post-loop read would fail at C
	// compile) -- the property under test is checker acceptance.
	accepted := []struct {
		name, src string
	}{
		{"j_forward", forScopeForward},
		{"p_forin_after", forScopeForInAfter},
		{"q_while_after", forScopeWhileAfter},
	}
	for _, tc := range accepted {
		t.Run(tc.name+"_check", func(t *testing.T) {
			path := forScopeSource(t, tc.name, tc.src)
			for _, engine := range []string{"go", "kcc"} {
				t.Run(engine, func(t *testing.T) {
					out, errOut, code := forScopeInvoke(t, engine, "check", path, false)
					if code != 0 {
						t.Errorf("%s check exit = %d, want 0\nstdout:\n%s\nstderr:\n%s", engine, code, out, errOut)
					}
				})
			}
		})
	}

	// Run goldens: ending the for scope must not change any value computed
	// INSIDE the loop or any outer binding read after it.
	goldens := []struct {
		name, src, want string
	}{
		{"n_valid_all", `func main() {
    let i = 99
    for (let i = 0; i < 2; i = i + 1) {
        print(i)
    }
    print(i)
    let k = 0
    for (; k < 3; k = k + 1) {
        print(k)
    }
    print(k)
    for (let n = 0; n < 2; n = n + 1) {
        for (let n = 5; n < 6; n = n + 1) {
            print(n)
        }
        print(n)
    }
    for (let m = 0; m < 3; m = m + 1) {
        let d = m + 10
        print(d)
    }
}
`, "0\n1\n99\n0\n1\n2\n3\n5\n0\n5\n1\n10\n11\n12\n"},
		{"r_nested", `func main() {
    for (let n = 0; n < 2; n = n + 1) {
        for (let n = 5; n < 6; n = n + 1) {
            print(n)
        }
        print(n)
    }
}
`, "5\n0\n5\n1\n"},
		{"s_emptyinit", `func main() {
    let k = 0
    for (; k < 3; k = k + 1) {
        print(k)
    }
    print(k)
}
`, "0\n1\n2\n3\n"},
		{"t_shadow", `func main() {
    let i = 99
    for (let i = 0; i < 2; i = i + 1) {
        print(i)
    }
    print(i)
}
`, "0\n1\n99\n"},
		{"o_valid_bodylet", `func main() {
    let total = 0
    for (let i = 0; i < 4; i = i + 1) {
        let sq = i * i
        total = total + sq
    }
    print(total)
}
`, "14\n"},
	}
	norm := func(s string) string { return strings.ReplaceAll(s, "\r\n", "\n") }
	for _, tc := range goldens {
		t.Run(tc.name, func(t *testing.T) {
			path := forScopeSource(t, tc.name, tc.src)
			// This gate measures CHECKER scoping parity, so pin the C23 target.
			// Since v1.2.35 slice 2 an untargeted kcc run may choose native,
			// whose documented v1 shadowing semantics are deliberately
			// different from the C23 backend.
			goOut, goErr, goCode := forScopeInvoke(t, "go", "run", path, true, "--target", "c23")
			if goCode != 0 {
				t.Fatalf("go run exit = %d, want 0\nstdout:\n%s\nstderr:\n%s", goCode, goOut, goErr)
			}
			kccOut, kccErr, kccCode := forScopeInvoke(t, "kcc", "run", path, true, "--target", "c23")
			if kccCode != 0 {
				t.Fatalf("kcc run exit = %d, want 0\nstdout:\n%s\nstderr:\n%s", kccCode, kccOut, kccErr)
			}
			if got := norm(goOut); got != tc.want {
				t.Errorf("go stdout = %q, want %q", got, tc.want)
			}
			if norm(kccOut) != norm(goOut) {
				t.Errorf("engine stdout differs:\ngo:  %q\nkcc: %q", norm(goOut), norm(kccOut))
			}
		})
	}
}

// A gate that asserts nothing if the CLI could not be built is not evidence.
func TestPhaseForScope_HarnessIsNonVacuous(t *testing.T) {
	cli, kcc := forScopePrepare(t)
	if _, err := os.Stat(cli); err != nil {
		t.Fatalf("CLI missing: %v", err)
	}
	if _, err := os.Stat(kcc); err != nil {
		t.Fatalf("fresh kcc missing: %v", err)
	}
	if fmt.Sprintf("%v", cli) == fmt.Sprintf("%v", kcc) {
		t.Fatal("CLI and kcc resolved to the same binary")
	}
}
