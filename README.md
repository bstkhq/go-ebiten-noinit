# go-ebiten-noinit

`go-ebiten-noinit` lets a command-line process link a package which imports
[Ebitengine](https://ebitengine.org) without initializing its graphical UI.

Import it for its side effects from the program which only needs the indirect
dependency:

```go
import (
	_ "github.com/bstkhq/go-ebiten-noinit"

	"example.com/a/package/which/indirectly/imports/ebiten"
)
```

The `github.com/bstkhq/go-ebiten-noinit` import path is part of the mechanism: it
sorts before `github.com/hajimehoshi/ebiten/v2`, so Go initializes this package
first. Renaming or publishing it under a path which sorts later would break
that guarantee. The package must be linked together with Ebitengine; it is not
useful as the only import in a binary.

On Linux and BSD, importing this package is an explicit opt-in: it uses
`go:linkname` to replace Ebitengine's `internal/ui` initializer in Go's
`inittask` unconditionally. `DISPLAY` is deliberately ignored. The replacement
creates only the minimal in-memory UI state needed by other package
initializers and never initializes GLFW, X11, or a graphics driver. Other
operating systems are untouched.

This replaces the one-line change previously carried in the
[erparts/ebiten fork](https://github.com/erparts/ebiten/commit/6011b342019d9cd2ccd6aa1abce065828746dcb2)
without replacing the whole Ebitengine module.

## Limits

This is not a headless Ebitengine backend. A binary which imports this package
must never call `ebiten.RunGame` or any API which needs the UI, even when
`DISPLAY` is set. Do not import it from binaries which can run graphically. It
is only for commands which link Ebitengine accidentally and never use it.

The link targets and `inittask` layout are compiler and Ebitengine internals.
This module supports Ebitengine v2.9.9 and newer, including prereleases. It is
currently checked against v2.9.9 and v2.10.0-alpha.11 with Go 1.26.5, and
requires Go 1.24 or newer. Upgrading Ebitengine or Go should include running the
subprocess test before release.

## License

MIT. See [LICENSE](LICENSE).
