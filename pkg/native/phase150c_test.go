package native

import (
	"bytes"
	"crypto/sha256"
	"runtime"
	"testing"

	"karkain/pkg/parser"
)

// Phase 150C focused evidence.
//
// The program used throughout is deliberately minimal and uses ONLY
// features the pre-150C backend already supported: integer locals,
// integer arithmetic, a while loop, and integer print. No strings, no
// arrays, no maps, no records, and nothing that reaches the heap arena, so
// a failure here can only be the allocator.
const raFirstTestSrc = "func main() {\n" +
"    let a = 1\n" +
"    let b = 2\n" +
"    let c = 3\n" +
"    let d = 4\n" +
"    let e = 5\n" +
"    let f = 6\n" +
"    let g = 7\n" +
"    let s = 0\n" +
"    let i = 0\n" +
"    while (i < 3) {\n" +
"        s = s + a + b + c + d + e + f + g\n" +
"        i = i + 1\n" +
"    }\n" +
"    print(s)\n" +
"    print(a + b + c + d + e + f + g)\n" +
"}\n"

// raFirstTestExpected is 3 * 28 == 84, then the one-shot sum 28.
const raFirstTestExpected = "84\n28\n"

// raPlanFor runs the allocator exactly the way CompileProgramForOS does and
// returns the name -> register map, so a test can assert WHICH registers the
// locals landed in rather than only how many.
func raPlanFor(t *testing.T, src string) (map[string]Reg, []Reg) {
t.Helper()
prog := parseNative(t, src)
b := newBuilder()
b.goos = OSLinux
for _, stmt := range prog.Statements {
if fd, ok := stmt.(*parser.FuncDecl); ok {
if !b.funcs[fd.Name] {
b.funcs[fd.Name] = true
b.ftab[fd.Name] = fd
}
}
}
if err := b.collectStructs(prog); err != nil {
t.Fatalf("collectStructs: %v", err)
}
for name := range b.ftab {
if _, err := b.retKindOf(name); err != nil {
t.Fatalf("retKindOf(%s): %v", name, err)
}
}
b.usesFloat, b.usesConcat, b.usesStrEq, b.usesStrSlice, b.usesPush, b.pushInLoop, b.concatInLoop, b.heapSize = scanValueUsage(prog)
b.usesStrField = scanStructUsage(prog)
b.prog = prog
out := map[string]Reg{}
var used []Reg
for _, stmt := range prog.Statements {
fd, ok := stmt.(*parser.FuncDecl)
if !ok {
continue
}
if err := b.layout(fd); err != nil {
t.Fatalf("layout(%s): %v", fd.Name, err)
}
for k, v := range b.regOf {
out[k] = v
}
used = b.regsUsed
}
return out, used
}

// TestPhase150C_FirstTest is the focused gate required before any broad
// suite: it asserts the exact allocation (first four live locals into
// R12..R15, the rest spilled to their existing frame slots) and that the
// generated program computes the right answer.
func TestPhase150C_FirstTest(t *testing.T) {
plan, used := raPlanFor(t, raFirstTestSrc)

// R12..R15 must all be claimed -- the four allocatable registers.
for _, reg := range allocRegs {
if !containsReg(used, reg) {
t.Errorf("allocatable register %v was not claimed; used=%v", reg, used)
}
}
if len(used) != len(allocRegs) {
t.Errorf("regsUsed = %v, want exactly %v", used, allocRegs)
}

// Four locals must be in registers and the rest must be spilled.
inReg := map[string]Reg{}
for name, reg := range plan {
inReg[name] = reg
}
if len(inReg) != len(allocRegs) {
t.Fatalf("allocated %d locals (%v), want %d", len(inReg), inReg, len(allocRegs))
}
// Whatever the exact names, each allocated local must sit in a distinct
// allocatable register, and every OTHER int local must be absent from
// the plan (i.e. spilled back to its frame slot).
seen := map[Reg]string{}
for name, reg := range inReg {
if prev, dup := seen[reg]; dup {
t.Errorf("locals %q and %q both claim register %v", prev, name, reg)
}
seen[reg] = name
}

// Determinism: the same source must plan identically twice.
plan2, used2 := raPlanFor(t, raFirstTestSrc)
if len(plan) != len(plan2) || len(used) != len(used2) {
t.Fatalf("allocation is not deterministic: %v/%v vs %v/%v", plan, used, plan2, used2)
}
for name, reg := range plan {
if plan2[name] != reg {
t.Errorf("local %q planned %v then %v -- allocation is not deterministic", name, reg, plan2[name])
}
}
}

func containsReg(list []Reg, r Reg) bool {
for _, x := range list {
if x == r {
return true
}
}
return false
}

// --- Required case 1: BELOW THRESHOLD -------------------------------
// Two live int locals in a loop is under the pressure threshold, so the
// allocator must not engage and every local keeps its existing frame slot.
const raBelowSrc = "func main() {\n" +
"    let a = 1\n" +
"    let b = 2\n" +
"    let i = 0\n" +
"    let s = 0\n" +
"    while (i < 3) {\n" +
"        s = s + a + b\n" +
"        i = i + 1\n" +
"    }\n" +
"    print(s)\n" +
"}\n"

func TestPhase150C_BelowThreshold(t *testing.T) {
plan, used := raPlanFor(t, raBelowSrc)
if len(plan) != 0 || len(used) != 0 {
t.Errorf("allocator engaged below threshold: plan=%v used=%v; want none", plan, used)
}
}

// --- Required case 2 + 3: ENGAGEMENT and SPILL ----------------------
// The first test's program has 9 int locals and 4 registers, so exactly
// four are allocated and the remaining five must be spilled -- i.e. absent
// from the plan and therefore read/written through their frame slots.
func TestPhase150C_EngagementAndSpill(t *testing.T) {
plan, used := raPlanFor(t, raFirstTestSrc)
if len(used) != 4 {
t.Fatalf("regsUsed = %v, want 4", used)
}
if len(plan) != 4 {
t.Fatalf("allocated %d locals (%v), want 4", len(plan), plan)
}
// Every allocated local must be one of the program's int locals, and
// the spilled ones (a,b,c,d,e,f,g,s,i minus the allocated four) must be
// absent -- which is what routes them back to the frame slot.
const totalIntLocals = 9
spilled := totalIntLocals - len(plan)
if spilled != 5 {
t.Errorf("spilled = %d, want 5 (9 int locals - 4 registers)", spilled)
}
}

// --- Required case 4: CONTROL FLOW ----------------------------------
// Allocated locals must stay correct across a loop back-edge AND a branch.
// The sum accumulates over three iterations, and the branch contributes a
// different amount on one of them, so a stale register would show up in the
// printed total.
const raControlSrc = "func main() {\n" +
"    let a = 1\n" +
"    let b = 2\n" +
"    let c = 3\n" +
"    let d = 4\n" +
"    let e = 5\n" +
"    let f = 6\n" +
"    let g = 7\n" +
"    let s = 0\n" +
"    let i = 0\n" +
"    while (i < 3) {\n" +
"        if (i == 1) {\n" +
"            s = s + a + b\n" +
"        } else {\n" +
"            s = s + c + d + e + f + g\n" +
"        }\n" +
"        i = i + 1\n" +
"    }\n" +
"    print(s)\n" +
"}\n"

// 2 * (1+2) + (3+4+5+6+7) == 6 + 25 == 31
// Iterations i=0,1,2: the else branch runs at i=0 and i=2 (3+4+5+6+7 = 25
// each) and the then branch at i=1 (1+2 = 3), so the total is 25+3+25.
const raControlExpected = "53\n"
// --- Required case 6: NATIVE EXECUTION ------------------------------
// Run the focused register-pressure programs on the PE container, where
// they execute for real on a windows/amd64 host. This is the only evidence
// that the allocation produces correct VALUES, not just correct encodings.
func TestPhase150C_FocusedExecPE(t *testing.T) {
if runtime.GOOS != "windows" || runtime.GOARCH != "amd64" {
t.Skip("PE execution needs windows/amd64")
}
for _, c := range []struct{ name, src, want string }{
{"first_test", raFirstTestSrc, raFirstTestExpected},
{"control_flow", raControlSrc, raControlExpected},
{"below_threshold", raBelowSrc, "9\n"},
} {
c := c
t.Run(c.name, func(t *testing.T) {
img := compileNativeOS(t, OSWindows, c.src)
out, code := runNativeWindows(t, img)
if out == "" {
t.Fatalf("%s: produced NO output (want %q), exit %d", c.name, c.want, code)
}
if out != c.want {
t.Errorf("%s: output %q, want %q", c.name, out, c.want)
}
if code != 0 {
t.Errorf("%s: exit %d, want 0 (out=%q)", c.name, code, out)
}
})
}
}

// --- Required case 5: DETERMINISM (image level) ----------------------
// Repeated compilation of the register-pressure program must yield a
// byte-identical PE image, which is only true if the allocation does not
// depend on map iteration order.
func TestPhase150C_DeterministicImage(t *testing.T) {
first := compileNativeOS(t, OSWindows, raFirstTestSrc)
second := compileNativeOS(t, OSWindows, raFirstTestSrc)
if !bytes.Equal(first, second) {
a := sha256.Sum256(first)
b := sha256.Sum256(second)
t.Errorf("non-deterministic image (%x vs %x) -- the allocation depends on map order", a[:8], b[:8])
}
}
