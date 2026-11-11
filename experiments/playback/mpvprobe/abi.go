// Package mpvprobe is the opt-in, fixture-only replacement feasibility adapter.
// It never loads a native library during package initialization or ordinary tests.
package mpvprobe

import (
	"runtime"
	"unsafe"
)

// Private mappings of mpv v0.42.0 client API 2.6. The supported native target is
// Darwin arm64: C int=4, pointers/size_t/unsigned long=8, double=8. Structs cross
// the ABI only by pointer. No native pointer or type enters the plain contract.
type event struct {
	ID    int32
	Error int32
	Reply uint64
	Data  unsafe.Pointer
}
type property struct {
	Name   *byte
	Format int32
	_      int32
	Data   unsafe.Pointer
}
type renderParam struct {
	Kind int32
	_    int32
	Data unsafe.Pointer
}

type nativeAPI struct {
	version      func() uint64
	create       func() uintptr
	option       func(uintptr, string, string) int32
	initialize   func(uintptr) int32
	observe      func(uintptr, uint64, string, int32) int32
	command      func(uintptr, uint64, unsafe.Pointer) int32
	wait         func(uintptr, float64) unsafe.Pointer
	destroy      func(uintptr)
	renderCreate func(*uintptr, uintptr, unsafe.Pointer) int32
	renderUpdate func(uintptr) uint64
	render       func(uintptr, unsafe.Pointer) int32
	renderFree   func(uintptr)
	unload       func()
}

func (a *nativeAPI) send(core uintptr, id uint64, args []string) int32 {
	storage := make([][]byte, len(args))
	argv := make([]*byte, len(args)+1)
	for i, arg := range args {
		storage[i] = append([]byte(arg), 0)
		argv[i] = &storage[i][0]
	}
	// command_async copies arguments before returning; Go buffers are never
	// retained by native code. These are argv values, never parsed command text.
	result := a.command(core, id, unsafe.Pointer(&argv[0]))
	runtime.KeepAlive(storage)
	runtime.KeepAlive(argv)
	return result
}
