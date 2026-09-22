package cli

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

// TestVSCodeExtension_DebugWiring (Phase 140) validates the debugger
// surface: the manifest advertises karkain.debug with activation and a
// debuggerPath setting, extension.js registers the command and drives a
// build-then-debug flow (build -g, startDebugging, cppdbg/gdb), and the
// launch/tasks templates exist as valid JSON with launch + attach configs
// and a pre-launch build task.
func TestVSCodeExtension_DebugWiring(t *testing.T) {
	dir := vscodeExtensionDir(t)

	raw := readT(t, filepath.Join(dir, "package.json"))
	var m vscodeManifest
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		t.Fatalf("package.json invalid: %v", err)
	}
	found := false
	for _, c := range m.Contributes.Commands {
		if c.Command == "karkain.debug" {
			found = true
		}
	}
	if !found {
		t.Error("manifest missing karkain.debug command")
	}
	activated := false
	for _, a := range m.Activation {
		if a == "onCommand:karkain.debug" {
			activated = true
		}
	}
	if !activated {
		t.Error("manifest missing onCommand:karkain.debug activation")
	}
	if _, ok := m.Contributes.Configuration.Properties["karkain.debuggerPath"]; !ok {
		t.Error("manifest missing karkain.debuggerPath setting")
	}

	ext := readT(t, filepath.Join(dir, "extension.js"))
	for _, want := range []string{
		"registerCommand('karkain.debug'",
		"startDebugging",
		"cppdbg",
		"MIMode",
		"'build', '-g'",
	} {
		if !strings.Contains(ext, want) {
			t.Errorf("extension.js missing debug wiring %q", want)
		}
	}

	for _, f := range []string{"launch.json", "tasks.json"} {
		raw := readT(t, filepath.Join(dir, f))
		if !strings.Contains(raw, "Phase 140") {
			t.Errorf("%s missing the Phase 140 template header", f)
		}
		// Templates are JSONC (VS Code tolerates // comments); strip
		// full-line comments before strict validation.
		var kept []string
		for _, line := range strings.Split(raw, "\n") {
			if strings.HasPrefix(strings.TrimSpace(line), "//") {
				continue
			}
			kept = append(kept, line)
		}
		var v interface{}
		if err := json.Unmarshal([]byte(strings.Join(kept, "\n")), &v); err != nil {
			t.Errorf("%s invalid JSON: %v", f, err)
		}
	}
	launch := readT(t, filepath.Join(dir, "launch.json"))
	for _, want := range []string{"configurations", "cppdbg", "preLaunchTask", "attach"} {
		if !strings.Contains(launch, want) {
			t.Errorf("launch.json missing %q", want)
		}
	}
	tasks := readT(t, filepath.Join(dir, "tasks.json"))
	if !strings.Contains(tasks, "karkain: build -g current file") {
		t.Error("tasks.json missing the -g build task")
	}
}
