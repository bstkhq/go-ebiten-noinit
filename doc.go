// Package noinit lets a command-line program link Ebitengine on Linux or
// BSD without initializing its graphical UI.
//
// Ebitengine initializes GLFW from an init function, so a binary which links it
// through an indirect dependency dies on startup without a display. Importing
// this package for its side effects replaces that function with a minimal,
// non-graphical initializer:
//
//	import _ "github.com/bstkhq/go-ebiten-noinit"
//
// There is nothing to call. DISPLAY is deliberately ignored: the import is an
// explicit declaration that the binary never needs Ebitengine's UI.
//
// This is not a headless renderer. After the UI initializer has been skipped,
// RunGame and every other API which needs the UI must not be called; doing so
// is not diagnosed and faults on a nil pointer inside Ebitengine. Do not import
// this package from a binary which can run graphically.
//
// Only the initializer which builds the UI is replaced. Ebitengine's others
// still run, including the runtime.LockOSThread call which pins the main
// goroutine to its thread for the life of the process.
//
// The package is a no-op on other operating systems, which is not the same as
// their being safe: Ebitengine drives GLFW on Windows and macOS too. It
// supports Ebitengine v2.8.0 and newer and requires Go 1.24 or newer. Older
// Ebitengine versions fail loudly at startup.
//
// The implementation depends on compiler, linker and Ebitengine internals.
// INTERNALS.md in the repository describes them, and what to check when
// upgrading either.
package noinit
