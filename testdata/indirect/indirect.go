// Package indirect is the dependency used by the subprocess regression test.
// It deliberately does not import ebiten-noinit: the executable imports this
// package and ebiten-noinit as siblings, just like a real program whose other
// dependency happens to pull Ebitengine in.
package indirect

import (
	"github.com/hajimehoshi/ebiten/v2"
	_ "github.com/hajimehoshi/ebiten/v2/ebitenutil"
	_ "github.com/hajimehoshi/ebiten/v2/vector"
)

// UIState returns enough public Ebitengine state to prove that its minimal UI
// object exists without initializing a graphics backend.
func UIState() (graphicsLibrary, unknownGraphicsLibrary int) {
	var info ebiten.DebugInfo
	ebiten.ReadDebugInfo(&info)
	return int(info.GraphicsLibrary), int(ebiten.GraphicsLibraryUnknown)
}
