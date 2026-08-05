//go:build (freebsd || (linux && !android) || netbsd || openbsd) && !nintendosdk && !playstation5

package noinit

import (
	"image"
	"reflect"
	"runtime"
	"unsafe"
)

// initTask is the header of runtime.initTask. The function PCs immediately
// follow it in memory. Keep this in sync with runtime/proc.go.
type initTask struct {
	state uint32
	nfns  uint32
}

// uiInitTask is Ebitengine's compiler-generated list of package initializers.
//
// This package must not import Ebitengine. Doing so would make Ebitengine its
// dependency, force the UI initializer to run first, and defeat this patch.
//
//go:linkname uiInitTask github.com/hajimehoshi/ebiten/v2/internal/ui..inittask
var uiInitTask initTask

// uiInit is the initializer in internal/ui/ui.go which creates the UI and
// initializes GLFW. The implementation belongs to Ebitengine.
//
//go:linkname uiInit github.com/hajimehoshi/ebiten/v2/internal/ui.init.0
func uiInit()

//go:linkname theUI github.com/hajimehoshi/ebiten/v2/internal/ui.theUI
var theUI unsafe.Pointer

// typesByString has an intentional linkname handshake in package reflect.
// Its real result is []*abi.Type; pointers have the same slice representation
// as unsafe.Pointer, which lets this package use it without importing an
// internal standard-library package.
//
//go:linkname typesByString reflect.typesByString
func typesByString(string) []unsafe.Pointer

func init() {
	// A completed task means Ebitengine initialized before this package. This
	// is possible when this package is loaded later from a plugin; changing an
	// already-consumed task would have no effect.
	if uiInitTask.state != 0 {
		return
	}

	if !replaceInitializer(&uiInitTask, functionPC(uiInit), functionPC(initializeWithoutUI)) {
		panic("noinit: Ebitengine's UI initializer was not found; unsupported Ebitengine or Go version")
	}
}

const maxInitializers = 1024

func replaceInitializer(task *initTask, target, replacement uintptr) bool {
	if task.nfns == 0 || task.nfns > maxInitializers {
		return false
	}

	first := unsafe.Add(unsafe.Pointer(task), unsafe.Sizeof(*task))
	var match *uintptr
	for i := uint32(0); i < task.nfns; i++ {
		slot := (*uintptr)(unsafe.Add(first, uintptr(i)*unsafe.Sizeof(uintptr(0))))
		if *slot == target {
			if match != nil {
				return false
			}
			match = slot
		}
	}
	if match == nil {
		return false
	}
	*match = replacement
	return true
}

func functionPC(fn func()) uintptr {
	return reflect.ValueOf(fn).Pointer()
}

// initializeWithoutUI reproduces the part of Ebitengine's
// newUserInterface which runs before (*UserInterface).init initializes GLFW.
//
// Ebitengine's vector package calls ebiten.NewImage while creating its
// package-level white image. Keeping a real, correctly typed zero UserInterface
// makes that load-time image allocation work without pretending that a graphics
// backend exists.
//
//go:noinline
func initializeWithoutUI() {
	u := reflect.New(userInterfaceType())

	setCleared := requiredMethod(u, "SetScreenClearedEveryFrame")
	setCleared.Call([]reflect.Value{reflect.ValueOf(true)})

	// GraphicsLibraryUnknown is 1 in the supported Ebitengine versions. Zero
	// means Auto, which would falsely report that backend selection had not
	// happened yet.
	graphicsLibrary := requiredField(u.Elem(), "graphicsLibrary")
	graphicsLibraryValue := reflect.NewAt(graphicsLibrary.Type(), unsafe.Pointer(graphicsLibrary.UnsafeAddr()))
	requiredMethod(graphicsLibraryValue, "Store").Call([]reflect.Value{reflect.ValueOf(int32(1))})

	newImage := requiredMethod(u, "NewImage")
	if newImage.Type().NumIn() != 3 {
		panic("noinit: unexpected Ebitengine NewImage signature")
	}
	imageTypeRegular := reflect.New(newImage.Type().In(2)).Elem()
	whiteImage := newImage.Call([]reflect.Value{
		reflect.ValueOf(3),
		reflect.ValueOf(3),
		imageTypeRegular,
	})[0]

	whiteImageField := requiredField(u.Elem(), "whiteImage")
	if whiteImageField.Type() != whiteImage.Type() {
		panic("noinit: Ebitengine UserInterface.whiteImage changed type")
	}
	reflect.NewAt(whiteImageField.Type(), unsafe.Pointer(whiteImageField.UnsafeAddr())).Elem().Set(whiteImage)

	pix := make([]byte, 4*3*3)
	for i := range pix {
		pix[i] = 0xff
	}
	requiredMethod(whiteImage, "WritePixels").Call([]reflect.Value{
		reflect.ValueOf(pix),
		reflect.ValueOf(image.Rect(0, 0, 3, 3)),
	})

	theUI = u.UnsafePointer()
	runtime.KeepAlive(u)
}

func requiredField(value reflect.Value, name string) reflect.Value {
	field := value.FieldByName(name)
	if !field.IsValid() || !field.CanAddr() {
		panic("noinit: Ebitengine field not found: " + name)
	}
	return field
}

func requiredMethod(value reflect.Value, name string) reflect.Value {
	method := value.MethodByName(name)
	if !method.IsValid() {
		panic("noinit: Ebitengine method not found: " + name)
	}
	return method
}

func userInterfaceType() reflect.Type {
	var found reflect.Type
	for _, pointer := range typesByString("*ui.UserInterface") {
		t := asReflectType(pointer)
		if t.Kind() != reflect.Pointer {
			continue
		}
		elem := t.Elem()
		if elem.Name() != "UserInterface" || elem.PkgPath() != "github.com/hajimehoshi/ebiten/v2/internal/ui" {
			continue
		}
		if found != nil {
			panic("noinit: Ebitengine UserInterface type was not unique")
		}
		found = elem
	}
	if found == nil {
		panic("noinit: Ebitengine UserInterface type was not found")
	}
	return found
}

// asReflectType turns the *abi.Type returned by reflect.typesByString into the
// public reflect.Type interface. Every runtime type is represented by the same
// *reflect.rtype implementation, so the itab from any reflect.Type is reusable;
// only its data pointer changes.
func asReflectType(pointer unsafe.Pointer) reflect.Type {
	template := reflect.TypeOf(0)
	words := *(*[2]unsafe.Pointer)(unsafe.Pointer(&template))
	words[1] = pointer
	return *(*reflect.Type)(unsafe.Pointer(&words))
}
