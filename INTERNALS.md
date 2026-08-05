# Internals

How `go-ebiten-noinit` works, what it assumes, and how it breaks. Read this
before changing it or upgrading Ebitengine. For what the package is and how to
use it, see the [README](README.md).

## The mechanism

The compiler emits an `inittask` record per package: a small header followed by
the addresses of that package's initializer functions. The runtime walks these
records before `main`.

This package declares Ebitengine's record and its UI initializer with
`go:linkname`, then rewrites the one entry that points at the initializer:

```go
//go:linkname uiInitTask github.com/hajimehoshi/ebiten/v2/internal/ui..inittask
var uiInitTask initTask

//go:linkname uiInit github.com/hajimehoshi/ebiten/v2/internal/ui.init.0
func uiInit()
```

The replacement reproduces `newUserInterface` up to, but not including, the
`u.init()` call that initializes GLFW. That keeps a real, correctly typed
`UserInterface` in `ui.theUI`, which is what lets package initializers that
allocate images at load time — `ebiten/vector` does — keep working.

The header layout has to match `runtime.initTask` in `runtime/proc.go`, and the
function pointers are read at a fixed offset of 8 bytes past it.

## Initialization order

The order is decided by the **linker**, not by the compiler and not by the
order of import statements. `cmd/link/internal/ld/inittask.go` sorts `inittask`
records topologically and breaks ties by symbol name, using a lexicographic
heap. `github.com/bstkhq/…` sorts before `github.com/hajimehoshi/…`, so this
package wins.

Two invariants keep that true:

1. **The import path must sort before Ebitengine's.** Republishing under a path
   that sorts later breaks the mechanism.

2. **Everything this package imports must be something Ebitengine's
   `internal/ui` already reaches.** A tie-break only happens between records
   that are both schedulable. If this package is waiting on a dependency that
   `internal/ui` does not have, `internal/ui` runs first and there is no tie to
   break.

The second one is one import away at all times: adding `net/http` to
`noinit_linbsd.go` is enough to hand the order back to Ebitengine.
`TestImportsStayAheadOfEbiten` compares the two dependency closures to enforce
it.

Losing the order is only loud without a display, where Ebitengine panics.
With a display it succeeds, and the package silently does nothing — the guard
in `init` sees a consumed `inittask` and returns.

## Why the replacement avoids reflection

The replacement calls three Ebitengine methods through `go:linkname`:

```go
//go:linkname uiNewImage github.com/hajimehoshi/ebiten/v2/internal/ui.(*UserInterface).NewImage
func uiNewImage(u unsafe.Pointer, width, height, imageType int) unsafe.Pointer
```

Reaching them through `reflect` does not work. A method that is only ever
reached by reflection looks unreferenced to the linker, so its dead code pass
prunes it: the method table entry is redirected to `runtime.unreachableMethod`
and the method type is dropped. Calling one aborts with `fatal error:
unreachable method called`; even inspecting one crashes inside `reflect` while
building the `Method`.

This is the common case, not an edge case. The fewer Ebitengine APIs the
program uses, the more the linker prunes — and a program that never uses
Ebitengine is exactly what this package is for.

Reflection is still used for what the linker never prunes: locating
`ui.UserInterface` through `reflect.typesByString`, and reading and writing
fields.

`go:linkname` does no type checking, so each call is verified by its effect on
a field instead of by its signature: `SetScreenClearedEveryFrame` must flip
`isScreenClearedEveryFrame`, and `NewImage` must return an image whose `width`,
`height` and `ui` fields are what was asked for.

## Why `init.0` is checked

Which `init` function the compiler names `init.0` depends on the order of the
file names in the package. A new file sorting before `ui.go` takes the name,
and the patch would land on an unrelated initializer while the real one still
opens X11.

This is not hypothetical: Ebitengine `v2.10.0-alpha.13` added
`internal/ui/api_darwin.go`, which took `init.0` on darwin. `verifyUIInitializer`
resolves the symbol back to a file through `runtime.FuncForPC` and refuses to
patch anything that is not `internal/ui/ui.go`. The file name survives
`-trimpath` and `-ldflags=-w`, both of which keep the pclntab.

## Failure modes

Everything shows up at build time or during startup, never as something subtly
wrong later. Runtime messages are panics prefixed `noinit:`, abbreviated `…`.

| Situation                                         | What you see                                                          |
| ------------------------------------------------- | --------------------------------------------------------------------- |
| Ebitengine is not in the binary                   | Link error: `relocation target …internal/ui.init.0 not defined`        |
| An Ebitengine method was renamed or removed       | Link error naming the missing symbol                                   |
| Ebitengine v2.7.x or older                        | `…isScreenClearedEveryFrame is not a sync/atomic value`                |
| `init.0` is no longer the UI initializer          | `…init.0 is declared in <file>, not /internal/ui/ui.go`                |
| An Ebitengine signature changed                   | `…NewImage ignored its size` / `…ignored its receiver`                 |
| An Ebitengine field changed                       | `…field not found: <name>` / `…graphicsLibrary changed type`           |
| The `inittask` layout changed                     | `…UI initializer was not found; unsupported Ebitengine or Go version`  |

## Testing

`TestBlankEbitenImport` is the one that is easy to get wrong. It links a
**separate** binary that imports Ebitengine and calls nothing, because that is
the only way to see what the linker prunes. Inside the main test binary the
other tests reach Ebitengine's API, which keeps those methods alive and hides
the failure.

The rest of the suite runs Ebitengine's initialization in subprocesses, since
the behaviour under test happens before any test can observe it.

Upgrading Ebitengine or Go should include a full `go test ./...`. Note that
`go vet ./...` skips `testdata`, so those packages need vetting by path.
