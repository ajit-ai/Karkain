// v1.2.27: `karkain coverage <file.kark>` reports FUNCTION coverage from the
// opt-in generated-C instrumentation. The runtime counts executions of each
// Karkain function via the same name string used by karkain_frame_enter and
// dumps name<TAB>count lines at exit. This is deliberately function coverage,
// not line coverage: the generated C carries a set_line only at function
// entry, so a line-coverage claim here would overstate what is measured.
package cli

import (
	"bufio"
	"fmt"
	"karkain/pkg/codegen"
	"karkain/pkg/lexer"
	"karkain/pkg/parser"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type CoverageFunction struct {
	Name  string
	Count int64
}

type CoverageReport struct {
	Program   string
	Functions []CoverageFunction
}

func (c *CoverageReport) RenderText() string {
	covered := 0
	for _, f := range c.Functions {
		if f.Count > 0 {
			covered++
		}
	}
	pct := 0
	if len(c.Functions) > 0 {
		pct = covered * 100 / len(c.Functions)
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "Karkain function coverage: %s\n", c.Program)
	for _, f := range c.Functions {
		if f.Count > 0 {
			fmt.Fprintf(&sb, "  covered   %s (%d calls)\n", f.Name, f.Count)
		} else {
			fmt.Fprintf(&sb, "  uncovered %s\n", f.Name)
		}
	}
	fmt.Fprintf(&sb, "functions: %d/%d (%d%%)\n", covered, len(c.Functions), pct)
	return sb.String()
}

func CoverageCommand(file string, verbose bool) CommandResult {
	if !strings.HasSuffix(file, ".kark") {
		return CommandResult{ExitCode: ExitUsage, Message: "Coverage Error: input must be a .kark file"}
	}
	src, err := os.ReadFile(file)
	if err != nil {
		return CommandResult{ExitCode: ExitUsage, Message: fmt.Sprintf("Coverage Error: %v", err)}
	}
	p := parser.New(lexer.New(string(src)))
	prog := p.ParseProgram()
	if len(p.Errors) > 0 || prog == nil {
		return CommandResult{ExitCode: ExitCompile,
			Message: fmt.Sprintf("Coverage Error: cannot parse %s: %v", file, p.Errors)}
	}
	names := []string{}
	for _, s := range prog.Statements {
		if fn, ok := s.(*parser.FuncDecl); ok {
			names = append(names, fn.Name)
		}
	}

	dump := filepath.Join(os.TempDir(), fmt.Sprintf("karkain-cov-%d.txt", time.Now().UnixNano()))
	defer os.Remove(dump)
	prev, had := os.LookupEnv("KARKAIN_COV_OUT")
	_ = os.Setenv("KARKAIN_COV_OUT", dump)
	defer func() {
		if had {
			_ = os.Setenv("KARKAIN_COV_OUT", prev)
		} else {
			_ = os.Unsetenv("KARKAIN_COV_OUT")
		}
	}()

	cfg := codegen.Config{Coverage: true, Stdout: os.Stdout, Stderr: os.Stderr}
	runResult := RunCommand(file, cfg, verbose)
	if runResult.ExitCode != ExitSuccess {
		return runResult
	}
	f, err := os.Open(dump)
	if err != nil {
		return CommandResult{ExitCode: ExitFailure,
			Message: fmt.Sprintf("Coverage Error: no coverage data produced: %v", err)}
	}
	counts := map[string]int64{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		parts := strings.SplitN(sc.Text(), "\t", 2)
		if len(parts) != 2 {
			continue
		}
		var n int64
		if _, err := fmt.Sscanf(parts[1], "%d", &n); err == nil {
			counts[parts[0]] += n
		}
	}
	_ = f.Close()
	if err := sc.Err(); err != nil {
		return CommandResult{ExitCode: ExitFailure, Message: fmt.Sprintf("Coverage Error: invalid dump: %v", err)}
	}

	report := CoverageReport{Program: file}
	for _, name := range names {
		report.Functions = append(report.Functions, CoverageFunction{Name: name, Count: counts[name]})
	}
	return CommandResult{ExitCode: ExitSuccess, Message: report.RenderText()}
}
