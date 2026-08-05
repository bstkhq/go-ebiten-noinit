//go:build (freebsd || (linux && !android) || netbsd || openbsd) && !nintendosdk && !playstation5

package noinit_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
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

// TestBlankEbitenImport links a separate binary which imports Ebitengine and
// calls nothing. This test package cannot stand in for it: it reaches
// Ebitengine through testdata/indirect, and that alone keeps the methods the
// replacement initializer needs out of the linker's dead code pass. A program
// which only links Ebitengine keeps nothing, and calling a pruned method
// aborts with "unreachable method called".
func TestBlankEbitenImport(t *testing.T) {
	if testing.Short() {
		t.Skip("linking a second binary is too slow for -short")
	}

	exe := filepath.Join(t.TempDir(), "blank")
	build := exec.Command("go", "build", "-o", exe, "./testdata/blank")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("building the blank-import program: %v\n%s", err, out)
	}

	cmd := exec.Command(exe)
	cmd.Env = withoutEnv(os.Environ(), "DISPLAY")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("a program which only links Ebitengine must still start: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "package loaded") {
		t.Fatalf("helper did not reach main; output:\n%s", out)
	}
}

// TestImportsStayAheadOfEbiten guards the invariant that decides the
// initialization order.
//
// Go's linker schedules inittasks in dependency order and breaks ties by
// symbol name. This package only wins that tie-break if it is schedulable by
// the time internal/ui is, which holds as long as everything it imports is
// something internal/ui already reaches. An import internal/ui does not have
// could leave this package waiting while internal/ui runs, and the failure
// would be silent on a machine that has a display.
func TestImportsStayAheadOfEbiten(t *testing.T) {
	if testing.Short() {
		t.Skip("listing dependencies shells out to go list")
	}

	const self = "github.com/bstkhq/go-ebiten-noinit"
	ebitenDeps := listDeps(t, "github.com/hajimehoshi/ebiten/v2/internal/ui")

	for _, dep := range listDeps(t, self) {
		if dep == self {
			continue
		}
		if !slices.Contains(ebitenDeps, dep) {
			t.Errorf("this package imports %q, which Ebitengine's internal/ui does not reach; "+
				"that can make it initialize after Ebitengine", dep)
		}
	}
}

func listDeps(t *testing.T, pkg string) []string {
	t.Helper()
	out, err := exec.Command("go", "list", "-deps", pkg).Output()
	if err != nil {
		t.Fatalf("listing the dependencies of %s: %v", pkg, err)
	}
	return strings.Fields(string(out))
}

// assertMinimalUI checks state which Ebitengine only ever reaches by
// initializing GLFW. It deliberately does not look at the graphics library:
// that is chosen in RunGame, so it reads as unknown whether or not this
// package did anything.
func assertMinimalUI(t *testing.T) {
	t.Helper()
	if !indirect.MonitorUnset() {
		t.Fatal("a monitor was selected, so Ebitengine initialized GLFW")
	}
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
