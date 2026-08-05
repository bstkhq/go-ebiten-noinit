# go-ebiten-noinit

Link [Ebitengine](https://ebitengine.org) into a command-line program without
letting it open a display.

## The problem

Ebitengine builds its user interface from an `init` function, before `main`
runs. That is fine for a game. It is not fine for a CLI tool, a test binary or
a server that happens to depend on a package that imports Ebitengine: the
process dies on startup, on any machine without a display.

```
glfw: X11: The DISPLAY environment variable is missing: a platform-specific error occurred
panic: glfw: The GLFW library is not initialized: the GLFW library is not initialized

goroutine 1 [running]:
github.com/hajimehoshi/ebiten/v2/internal/ui.init.0()
	.../internal/ui/ui.go:101 +0x4e
```

There is no build tag or environment variable to turn it off.

## Usage

```console
go get github.com/bstkhq/go-ebiten-noinit
```

Import it for its side effects from the program — not from a library — that
pulls Ebitengine in indirectly:

```go
import (
	_ "github.com/bstkhq/go-ebiten-noinit"

	"example.com/a/package/which/indirectly/imports/ebiten"
)
```

That is all. There is no API and nothing to call. `DISPLAY` is deliberately
ignored: the import *is* the statement that this binary never wants a UI.

## Limits

**This is not a headless Ebitengine backend.** `ebiten.RunGame`, and anything
else that needs the UI, must never be called. Nothing diagnoses it; the program
faults on a nil pointer inside Ebitengine. Do not import this package from a
binary that can run graphically.

Drawing still works as long as it only queues commands. Anything that has to
talk to a GPU does not:

```go
img := ebiten.NewImage(4, 4)
img.Fill(color.White)   // fine
img.At(0, 0)            // panics: ui: ReadPixels cannot be called before the game starts
```

## Compatibility

| GOOS                           | Behaviour                                 |
| ------------------------------ | ----------------------------------------- |
| `linux` (excluding Android)    | **Active** — UI initialization is skipped  |
| `freebsd`, `netbsd`, `openbsd` | **Active** — UI initialization is skipped  |
| everything else                | No-op — compiles and does nothing          |

A no-op is not the same as being safe: Ebitengine drives GLFW on Windows and
macOS too, and a headless binary hits the same problem there. Only Linux and
BSD are addressed.

|               | Ebitengine                              | Go                 |
| ------------- | --------------------------------------- | ------------------ |
| Supported     | `v2.8.x`, `v2.9.x`, `v2.10.0-alpha.x`   | 1.24, 1.25, 1.26   |
| Not supported | `v2.7.x` and older                      | 1.23 and older     |

Older Ebitengine refuses to start with a `noinit:` panic rather than silently
opening a display. Older Go is rejected by `go.mod`.

## How it works

Go records each package's initializers in a compiler-generated `inittask`. This
package reaches Ebitengine's with `go:linkname` and, before Ebitengine has run,
swaps the pointer to its UI initializer for a replacement. The replacement
builds the same `UserInterface` value Ebitengine would have, minus the call
that initializes GLFW, X11 and a graphics driver.

The import path is part of the mechanism: Go's linker breaks initialization
ties by symbol name, and `github.com/bstkhq/…` sorts before
`github.com/hajimehoshi/…`.

[INTERNALS.md](INTERNALS.md) covers the rest — the ordering rules this relies
on, why the replacement avoids reflection, and what happens when Ebitengine or
Go moves underneath it.

## License

MIT. See [LICENSE](LICENSE).
