//go:build (freebsd || (linux && !android) || netbsd || openbsd) && !nintendosdk && !playstation5

package noinit_test

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"

	_ "github.com/bstkhq/go-ebiten-noinit"
	"github.com/bstkhq/go-ebiten-noinit/testdata/indirect"
)

const (
	noDisplayHelperEnv  = "EBITEN_NOINIT_NO_DISPLAY_TEST_HELPER"
	setDisplayHelperEnv = "EBITEN_NOINIT_SET_DISPLAY_TEST_HELPER"
)

// TestImportWithoutDisplay uses another copy of this test binary because the
// behavior under test happens during package initialization, before a test (or
// TestMain) can recover or make assertions about it.
func TestImportWithoutDisplay(t *testing.T) {
	if os.Getenv(noDisplayHelperEnv) == "1" {
		assertMinimalUI(t)
		fmt.Print("package loaded")
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run=^TestImportWithoutDisplay$")
	cmd.Env = append(
		withoutEnv(os.Environ(), "DISPLAY", noDisplayHelperEnv, setDisplayHelperEnv),
		noDisplayHelperEnv+"=1",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("loading an indirect Ebitengine dependency without DISPLAY: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "package loaded") {
		t.Fatalf("helper did not reach the test; output:\n%s", out)
	}
	if strings.Contains(string(out), "glfw:") {
		t.Fatalf("explicit opt-in must not initialize GLFW; output:\n%s", out)
	}
}

// TestImportWithDisplayStillSkipsUI proves that importing ebiten-noinit is an
// explicit opt-in. DISPLAY is ignored and Ebitengine must not attempt X11.
func TestImportWithDisplayStillSkipsUI(t *testing.T) {
	if os.Getenv(setDisplayHelperEnv) == "1" {
		assertMinimalUI(t)
		fmt.Print("package loaded without opening X11")
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run=^TestImportWithDisplayStillSkipsUI$")
	cmd.Env = append(
		withoutEnv(os.Environ(), "DISPLAY", noDisplayHelperEnv, setDisplayHelperEnv),
		"DISPLAY=ebiten-noinit-must-not-be-opened",
		setDisplayHelperEnv+"=1",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("loading with DISPLAY set: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "package loaded without opening X11") {
		t.Fatalf("helper did not reach the test; output:\n%s", out)
	}
	if strings.Contains(string(out), "glfw:") {
		t.Fatalf("explicit opt-in must not initialize GLFW; output:\n%s", out)
	}
}

func assertMinimalUI(t *testing.T) {
	t.Helper()
	library, unknown := indirect.UIState()
	if library != unknown {
		t.Fatalf("graphics library = %d, want unknown (%d)", library, unknown)
	}
}

func withoutEnv(env []string, names ...string) []string {
	blocked := make(map[string]struct{}, len(names))
	for _, name := range names {
		blocked[name] = struct{}{}
	}

	result := make([]string, 0, len(env))
	for _, item := range env {
		name, _, _ := strings.Cut(item, "=")
		if _, ok := blocked[name]; !ok {
			result = append(result, item)
		}
	}
	return result
}
