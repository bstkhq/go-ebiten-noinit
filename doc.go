// Package noinit lets a command-line program link Ebitengine on Linux or
// BSD without initializing its graphical UI.
//
// Ebitengine initializes GLFW from an init function. Importing this package for
// its side effects unconditionally replaces that function with a minimal,
// non-graphical initializer:
//
//	import _ "github.com/bstkhq/go-ebiten-noinit"
//
// The import path is significant: it sorts before Ebitengine's import path, so
// Go initializes this package first. Moving the package to an import path which
// sorts after github.com/hajimehoshi/ebiten/v2 would break the mechanism.
//
// DISPLAY is deliberately ignored. Importing this package is an explicit
// declaration that the binary never needs Ebitengine's UI. It changes nothing
// on platforms where Ebitengine does not use X11.
//
// This package is deliberately only for binaries which happen to link
// Ebitengine through an indirect dependency. Ebitengine has no headless
// renderer: after its UI init has been skipped, RunGame and other UI APIs must
// not be called. Do not import this package from a graphical binary.
//
// The implementation reaches into compiler-generated Ebitengine initialization
// metadata with go:linkname. It supports Ebitengine v2.9.9 and newer and is
// verified against v2.9.9 and v2.10.0-alpha.11. It can require an update when
// either Ebitengine or Go changes that metadata.
package noinit
