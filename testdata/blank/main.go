// Command blank links Ebitengine and never calls it, which is the shape of
// program ebiten-noinit exists for. Nothing here references the Ebitengine
// methods the replacement initializer needs, so the linker is free to prune
// them; TestBlankEbitenImport links this as its own binary to check that it
// does not.
package main

import (
	"fmt"

	_ "github.com/bstkhq/go-ebiten-noinit"
	_ "github.com/hajimehoshi/ebiten/v2"
)

func main() {
	fmt.Print("package loaded")
}
