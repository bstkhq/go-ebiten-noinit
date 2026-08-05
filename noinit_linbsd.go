//go:build (freebsd || (linux && !android) || netbsd || openbsd) && !nintendosdk && !playstation5

package noinit

// The imports below must stay a subset of what internal/ui reaches. Go's
// linker schedules inittasks in dependency order and breaks ties by symbol
// name, which is what makes this package initialize before Ebitengine. An
// import which internal/ui does not have could leave this package
// unschedulable while internal/ui is ready, and silently reverse that order.
import (
	"image"
	"reflect"
	"runtime"
	"strings"
	"sync/atomic"
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

// The three Ebitengine methods below are called through go:linkname rather
// than through reflect. A method which is only ever reached by reflection is
// unreferenced as far as the linker is concerned: its dead code pass prunes
// it, redirects its method table entry to runtime.unreachableMethod and drops
// its method type. Calling one then aborts the process, and even inspecting
// one crashes inside reflect. That is exactly the shape of a binary this
// package exists for, one which links Ebitengine and never calls it.
//
// A renamed or removed method fails at link time. A changed signature would
// not, so initializeWithoutUI checks what each call did to the fields it is
// supposed to touch.

//go:linkname uiSetScreenClearedEveryFrame github.com/hajimehoshi/ebiten/v2/internal/ui.(*UserInterface).SetScreenClearedEveryFrame
func uiSetScreenClearedEveryFrame(u unsafe.Pointer, cleared bool)

//go:linkname uiNewImage github.com/hajimehoshi/ebiten/v2/internal/ui.(*UserInterface).NewImage
func uiNewImage(u unsafe.Pointer, width, height, imageType int) unsafe.Pointer

//go:linkname uiImageWritePixels github.com/hajimehoshi/ebiten/v2/internal/ui.(*Image).WritePixels
func uiImageWritePixels(i unsafe.Pointer, pix []byte, region image.Rectangle)

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

	target := functionPC(uiInit)
	verifyUIInitializer(target)

	if !replaceInitializer(&uiInitTask, target, functionPC(initializeWithoutUI)) {
		panic("noinit: Ebitengine's UI initializer was not found; unsupported Ebitengine or Go version")
	}
}

// uiInitFile is where Ebitengine declares the initializer this package
// replaces. Matched as a suffix, since the prefix is a module cache path under
// -trimpath and an absolute path without it. The directory is part of it so
// that some other ui.go cannot match.
const uiInitFile = "/internal/ui/ui.go"

// verifyUIInitializer checks that internal/ui.init.0 is still the init
// function declared in internal/ui/ui.go.
//
// Which init function the compiler names init.0 depends on the order of the
// file names in the package, so a new file sorting before ui.go silently takes
// the name. Ebitengine v2.10.0-alpha.13 added internal/ui/api_darwin.go and it
// did exactly that on darwin. Without this check the same change in the Linux
// and BSD build would move the patch onto an unrelated initializer, leaving the
// real one to open X11.
func verifyUIInitializer(pc uintptr) {
	fn := runtime.FuncForPC(pc)
	if fn == nil {
		panic("noinit: Ebitengine's UI initializer has no symbol information")
	}
	// The file name survives -trimpath and -ldflags=-w; both keep the pclntab.
	if file, _ := fn.FileLine(pc); !strings.HasSuffix(file, uiInitFile) {
		panic("noinit: " + fn.Name() + " is declared in " + file + ", not " + uiInitFile +
			"; unsupported Ebitengine version")
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
// Every linknamed call below is checked by its effect on a field rather than
// by its signature. Reflection cannot inspect these methods either: a pruned
// method has no method type left, so reflect.Value.MethodByName crashes
// dereferencing it. Fields are part of the struct descriptor and always there.
//
//go:noinline
func initializeWithoutUI() {
	u := reflect.New(userInterfaceType())
	uPointer := u.UnsafePointer()

	uiSetScreenClearedEveryFrame(uPointer, true)
	if atomicWordOf(u.Elem(), "isScreenClearedEveryFrame").IsZero() {
		panic("noinit: Ebitengine SetScreenClearedEveryFrame had no effect; unsupported Ebitengine version")
	}

	// GraphicsLibraryUnknown is 1 in the supported Ebitengine versions. Zero
	// means Auto, which would falsely report that backend selection had not
	// happened yet.
	graphicsLibrary := atomicWordOf(u.Elem(), "graphicsLibrary")
	if graphicsLibrary.Kind() != reflect.Int32 {
		panic("noinit: Ebitengine UserInterface.graphicsLibrary changed type")
	}
	atomic.StoreInt32((*int32)(unsafe.Pointer(graphicsLibrary.UnsafeAddr())), 1)

	// The third argument is atlas.ImageTypeRegular, the zero value of a named
	// int type.
	whiteImage := uiNewImage(uPointer, 3, 3, 0)
	if whiteImage == nil {
		panic("noinit: Ebitengine NewImage returned nothing")
	}

	whiteImageField := requiredField(u.Elem(), "whiteImage")
	whiteImageValue := reflect.NewAt(whiteImageField.Type().Elem(), whiteImage)
	verifyImage(whiteImageValue.Elem(), uPointer)
	reflect.NewAt(whiteImageField.Type(), unsafe.Pointer(whiteImageField.UnsafeAddr())).Elem().Set(whiteImageValue)

	pix := make([]byte, 4*3*3)
	for i := range pix {
		pix[i] = 0xff
	}
	uiImageWritePixels(whiteImage, pix, image.Rect(0, 0, 3, 3))

	theUI = uPointer
	runtime.KeepAlive(u)
}

// verifyImage checks that uiNewImage built the image it was asked for. An
// Ebitengine signature change would leave the arguments misplaced, and this is
// what notices before the damage spreads.
func verifyImage(img reflect.Value, owner unsafe.Pointer) {
	if requiredField(img, "width").Int() != 3 || requiredField(img, "height").Int() != 3 {
		panic("noinit: Ebitengine NewImage ignored its size; unsupported Ebitengine version")
	}
	if requiredField(img, "ui").UnsafePointer() != owner {
		panic("noinit: Ebitengine NewImage ignored its receiver; unsupported Ebitengine version")
	}
}

// atomicWordOf returns the word a sync/atomic wrapper field stores. The
// wrappers are a zero-size noCopy marker plus a single value; looking for the
// value avoids depending on its name or on where the marker sits.
//
// Reaching for (*atomic.Int32).Store through reflect is not an option, for the
// same reason the Ebitengine methods are linknamed.
func atomicWordOf(owner reflect.Value, name string) reflect.Value {
	field := requiredField(owner, name)
	if field.Kind() != reflect.Struct {
		panic("noinit: Ebitengine UserInterface." + name + " is not a sync/atomic value")
	}

	var word reflect.Value
	for i := 0; i < field.NumField(); i++ {
		f := field.Field(i)
		if f.Type().Size() == 0 {
			continue
		}
		if word.IsValid() {
			panic("noinit: unexpected sync/atomic layout for " + name)
		}
		word = f
	}
	if !word.IsValid() {
		panic("noinit: unexpected sync/atomic layout for " + name)
	}
	return word
}

func requiredField(value reflect.Value, name string) reflect.Value {
	field := value.FieldByName(name)
	if !field.IsValid() || !field.CanAddr() {
		panic("noinit: Ebitengine field not found: " + name)
	}
	return field
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
