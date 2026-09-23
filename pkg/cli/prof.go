package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"karkain/pkg/codegen"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Prof format identifiers, accepted by `karkain prof --format`.
const (
	ProfFormatText   = "text"
	ProfFormatJSON   = "json"
	ProfFormatFolded = "folded"
)

// Profile is the canonical, deterministic result of `karkain prof`. It is the
// machine-readable contract (schema "karkain-profile-v1"). Timing fields are
// wall-clock nanoseconds aggregated by the embedded profiler runtime.
type Profile struct {
	Schema     string            `json:"schema"`
	Version    int               `json:"version"`
	Program    string            `json:"program"`
	Engine     string            `json:"engine"`
	DurationNS int64             `json:"duration_ns"`
	Overflow   bool              `json:"overflow"`
	Allocation ProfileAllocation `json:"allocation"`
	Functions  []ProfileFunction `json:"functions"`
	Calls      []ProfileCall     `json:"calls"`
	Folded     []ProfileFolded   `json:"folded"`
}

// ProfileAllocation tracks allocations performed by the generated program
// code (struct instances / raw memory ops). Peak is the maximum bytes held
// live at once among those tracked allocations. Cells counts Value-cell
// constructions (make_string/make_array/make_map in user code, Phase 141).
type ProfileAllocation struct {
	Count     int64 `json:"count"`
	Bytes     int64 `json:"bytes"`
	PeakBytes int64 `json:"peak_bytes"`
	Cells     int64 `json:"cells"`
}

// ProfileFunction is per-function aggregate timing. Total/self are inclusive
// and exclusive wall time; avg = total / calls (rounded down).
type ProfileFunction struct {
	Name    string `json:"name"`
	Calls   int64  `json:"calls"`
	TotalNS int64  `json:"total_ns"`
	SelfNS  int64  `json:"self_ns"`
	MinNS   int64  `json:"min_ns"`
	MaxNS   int64  `json:"max_ns"`
	AvgNS   int64  `json:"avg_ns"`
}

// ProfileCall is a caller->callee call-graph edge.
type ProfileCall struct {
	Caller  string `json:"caller"`
	Callee  string `json:"callee"`
	Count   int64  `json:"count"`
	TotalNS int64  `json:"total_ns"`
}

// ProfileFolded is one folded (flame-graph) stack path with its accumulated
// wall time. Paths use ";" separators, e.g. "main;fibonacci;fibonacci".
type ProfileFolded struct {
	Path string `json:"path"`
	NS   int64  `json:"ns"`
}

// profDump is the raw JSON document written by the generated C profiler.
type profDump struct {
	DurationNS int64 `json:"duration_ns"`
	Overflow   int   `json:"overflow"`
	Allocation struct {
		Count     int64 `json:"count"`
		Bytes     int64 `json:"bytes"`
		PeakBytes int64 `json:"peak_bytes"`
		Cells     int64 `json:"cells"`
	} `json:"allocation"`
	Functions []struct {
		Name    string `json:"name"`
		Calls   int64  `json:"calls"`
		TotalNS int64  `json:"total_ns"`
		SelfNS  int64  `json:"self_ns"`
		MinNS   int64  `json:"min_ns"`
		MaxNS   int64  `json:"max_ns"`
		AvgNS   int64  `json:"avg_ns"`
	} `json:"functions"`
	Calls []struct {
		Caller  string `json:"caller"`
		Callee  string `json:"callee"`
		Count   int64  `json:"count"`
		TotalNS int64  `json:"total_ns"`
	} `json:"calls"`
	Folded []struct {
		Path string `json:"path"`
		NS   int64  `json:"ns"`
	} `json:"folded"`
}

// ProfCommand compiles and runs targetFile with profiling instrumentation
// enabled, reads the deterministic dump written by the program, and renders
// the report in the requested format (text|json|folded). Profiling is an
// opt-in Go-engine capability: kcc and WASM targets are explicitly
// unsupported (no silent fallback). The program's normal stdout/stderr pass
// through to the terminal exactly as `karkain run` would.
func ProfCommand(targetFile, format, output, engine, target string, verbose bool) CommandResult {
	if format == "" {
		format = ProfFormatText
	}
	switch format {
	case ProfFormatText, ProfFormatJSON, ProfFormatFolded:
	default:
		return CommandResult{ExitCode: ExitUsage,
			Message: fmt.Sprintf("Profiling Error: unknown --format %q (supported: text, json, folded)", format)}
	}

	if engine != "" && !strings.EqualFold(engine, "go") {
		return CommandResult{ExitCode: ExitFailure,
			Message: "Profiling Error: karkain prof supports the Go engine only; the self-hosted kcc engine is not yet profiler-aware (deferred, Phase 110 boundary)."}
	}

	if target != "" && target != "native" && target != "c23" {
		return CommandResult{ExitCode: ExitFailure,
			Message: fmt.Sprintf("Profiling Error: could not profile target %q: profiling is unsupported/deferred for this target (no silent fallback; WASM profiling is deferred).", target)}
	}

	if err := ValidateKarFile(targetFile); err != nil {
		return CommandResult{ExitCode: ExitUsage, Message: err.Error()}
	}

	// Unique, cleanup-later dump path handed to the program via env.
	dumpFile, err := makeProfDumpPath()
	if err != nil {
		return CommandResult{ExitCode: ExitFailure, Message: fmt.Sprintf("Profiling Error: %v", err)}
	}
	prev := os.Getenv("KARKAIN_PROF_OUT")
	os.Setenv("KARKAIN_PROF_OUT", dumpFile)
	defer func() {
		if prev == "" {
			os.Unsetenv("KARKAIN_PROF_OUT")
		} else {
			os.Setenv("KARKAIN_PROF_OUT", prev)
		}
		os.Remove(dumpFile)
	}()

	cfg := codegen.Config{Profiling: true, Stdout: os.Stdout, Stderr: os.Stderr}
	if target != "" {
		cfg.Target = target
	}

	runResult := RunCommand(targetFile, cfg, verbose)
	if runResult.ExitCode != ExitSuccess {
		return runResult
	}

	data, err := os.ReadFile(dumpFile)
	if err != nil {
		return CommandResult{ExitCode: ExitFailure,
			Message: fmt.Sprintf("Profiling Error: no profile data produced (missing dump): %v", err)}
	}
	prof, err := parseProfDump(data, targetFile)
	if err != nil {
		return CommandResult{ExitCode: ExitFailure, Message: fmt.Sprintf("Profiling Error: invalid profile dump: %v", err)}
	}

	var rendered string
	switch format {
	case ProfFormatJSON:
		rendered, err = prof.RenderJSON()
	case ProfFormatFolded:
		rendered = prof.RenderFolded()
	default:
		rendered = prof.RenderText()
	}
	if err != nil {
		return CommandResult{ExitCode: ExitFailure, Message: fmt.Sprintf("Profiling Error: %v", err)}
	}

	if output == "" {
		fmt.Print(rendered)
	} else {
		if werr := os.WriteFile(output, []byte(rendered), 0644); werr != nil {
			return CommandResult{ExitCode: ExitFailure,
				Message: fmt.Sprintf("Profiling Error: could not write report: %v", werr)}
		}
	}
	return CommandResult{ExitCode: ExitSuccess, Message: ""}
}

func makeProfDumpPath() (string, error) {
	f, err := os.CreateTemp("", "karkain-prof-*.json")
	if err != nil {
		return "", err
	}
	name := f.Name()
	f.Close()
	os.Remove(name)
	return name, nil
}

// parseProfDump converts the profiler's raw document into the canonical
// Profile. Program is the base file name (never an absolute temp path) so
// reports are stable. Function/call/path order is preserved from the dump
// (deterministic source-order aggregation).
func parseProfDump(data []byte, targetFile string) (*Profile, error) {
	var d profDump
	if err := json.Unmarshal(data, &d); err != nil {
		return nil, err
	}
	p := &Profile{
		Schema:     "karkain-profile-v1",
		Version:    1,
		Program:    filepath.Base(targetFile),
		Engine:     "go",
		DurationNS: d.DurationNS,
		Overflow:   d.Overflow != 0,
		Allocation: ProfileAllocation{Count: d.Allocation.Count, Bytes: d.Allocation.Bytes, PeakBytes: d.Allocation.PeakBytes, Cells: d.Allocation.Cells},
	}
	for _, f := range d.Functions {
		p.Functions = append(p.Functions, ProfileFunction{
			Name: f.Name, Calls: f.Calls, TotalNS: f.TotalNS, SelfNS: f.SelfNS,
			MinNS: f.MinNS, MaxNS: f.MaxNS, AvgNS: f.AvgNS,
		})
	}
	for _, c := range d.Calls {
		p.Calls = append(p.Calls, ProfileCall{Caller: c.Caller, Callee: c.Callee, Count: c.Count, TotalNS: c.TotalNS})
	}
	for _, fp := range d.Folded {
		p.Folded = append(p.Folded, ProfileFolded{Path: fp.Path, NS: fp.NS})
	}
	return p, nil
}

// RenderJSON marshals the profile document deterministically.
func (p *Profile) RenderJSON() (string, error) {
	b, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return "", err
	}
	return string(b) + "\n", nil
}

// RenderFolded renders one "path ns" line per folded stack, sorted by path.
func (p *Profile) RenderFolded() string {
	var sb strings.Builder
	sorted := make([]ProfileFolded, len(p.Folded))
	copy(sorted, p.Folded)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Path < sorted[j].Path })
	for _, f := range sorted {
		fmt.Fprintf(&sb, "%s %d\n", f.Path, f.NS)
	}
	return sb.String()
}

// RenderText renders the human-readable report: header, function table (by
// total time), call graph, and allocation metrics.
func (p *Profile) RenderText() string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "Profile of %s (engine: %s, total: %s)\n", p.Program, p.Engine, nsString(p.DurationNS))
	if p.Overflow {
		sb.WriteString("note: profiler capacity exceeded; counts are truncated\n")
	}
	sb.WriteString("Functions (by total time):\n")
	sb.WriteString(fmt.Sprintf("  %-22s %10s %10s %10s %10s %10s %10s\n", "FUNCTION", "CALLS", "TOTAL", "SELF", "MIN", "MAX", "AVG"))
	fns := make([]ProfileFunction, len(p.Functions))
	copy(fns, p.Functions)
	sort.Slice(fns, func(i, j int) bool {
		if fns[i].TotalNS != fns[j].TotalNS {
			return fns[i].TotalNS > fns[j].TotalNS
		}
		return fns[i].Name < fns[j].Name
	})
	for _, f := range fns {
		fmt.Fprintf(&sb, "  %-22s %10d %10s %10s %10s %10s %10s\n",
			f.Name, f.Calls, nsString(f.TotalNS), nsString(f.SelfNS),
			nsString(f.MinNS), nsString(f.MaxNS), nsString(f.AvgNS))
	}
	sb.WriteString("Call graph:\n")
	edges := make([]ProfileCall, len(p.Calls))
	copy(edges, p.Calls)
	sort.Slice(edges, func(i, j int) bool {
		if edges[i].Caller != edges[j].Caller {
			return edges[i].Caller < edges[j].Caller
		}
		return edges[i].Callee < edges[j].Callee
	})
	for _, c := range edges {
		fmt.Fprintf(&sb, "  %s -> %s x%d (%s)\n", c.Caller, c.Callee, c.Count, nsString(c.TotalNS))
	}
	fmt.Fprintf(&sb, "Allocation: %d allocation(s), %s allocated, %s peak live, %d value cell(s)\n",
		p.Allocation.Count, nsString(p.Allocation.Bytes), nsString(p.Allocation.PeakBytes), p.Allocation.Cells)
	return sb.String()
}

// nsString renders a nanosecond count in a read-friendly unit.
func nsString(ns int64) string {
	switch {
	case ns < 1000:
		return fmt.Sprintf("%dns", ns)
	case ns < 1000*1000:
		return fmt.Sprintf("%.2fus", float64(ns)/1000.0)
	case ns < 1000*1000*1000:
		return fmt.Sprintf("%.2fms", float64(ns)/1e6)
	default:
		return fmt.Sprintf("%.2fs", float64(ns)/1e9)
	}
}

// writeProfileReport is a small helper used by tests to render a report to a
// writer without going through stdout.
func writeProfileReport(w io.Writer, format string, p *Profile) error {
	switch format {
	case ProfFormatJSON:
		b, err := json.MarshalIndent(p, "", "  ")
		if err != nil {
			return err
		}
		_, err = w.Write(append(b, '\n'))
		return err
	case ProfFormatFolded:
		_, err := io.WriteString(w, p.RenderFolded())
		return err
	default:
		_, err := io.WriteString(w, p.RenderText())
		return err
	}
}
