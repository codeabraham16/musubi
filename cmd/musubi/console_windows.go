//go:build windows

package main

import (
	"syscall"
	"unsafe"
)

// console_windows.go prepara la consola de Windows para que la salida del CLI se
// vea bien en el PRIMER contacto (doble clic / cmd.exe fresco): sin esto, los bytes
// UTF-8 que emite Musubi (✓, acentos) salen como mojibake en el codepage OEM
// (cp850/437) y las secuencias ANSI de color aparecen como basura. Es 100% Go
// (syscall a kernel32, sin CGo) y best-effort: si algo falla, se sigue sin tocar nada.

// vtEnabled indica si se logró habilitar el procesamiento de secuencias ANSI
// (Virtual Terminal). Si queda en false (consola muy vieja o salida redirigida), el
// helper de estilo cae a texto plano en vez de escupir códigos de escape crudos.
var vtEnabled bool

func init() {
	const (
		cpUTF8                          = 65001
		stdOutputHandle                 = 0xFFFFFFF5 // (DWORD)-11
		enableVirtualTerminalProcessing = 0x0004
		invalidHandle                   = ^uintptr(0)
	)
	kernel32 := syscall.NewLazyDLL("kernel32.dll")

	// 1) Codepage de salida = UTF-8: los ✓ y acentos renderizan bien en vez de mojibake.
	_, _, _ = kernel32.NewProc("SetConsoleOutputCP").Call(uintptr(cpUTF8))

	// 2) Habilitar VT processing en stdout para que el color ANSI se interprete (Win10+).
	h, _, _ := kernel32.NewProc("GetStdHandle").Call(uintptr(stdOutputHandle))
	if h == 0 || h == invalidHandle {
		return
	}
	var mode uint32
	getMode := kernel32.NewProc("GetConsoleMode")
	if ret, _, _ := getMode.Call(h, uintptr(unsafe.Pointer(&mode))); ret == 0 {
		return // stdout no es una consola real (pipe/redirección) → nada que habilitar
	}
	setMode := kernel32.NewProc("SetConsoleMode")
	if ret, _, _ := setMode.Call(h, uintptr(mode|enableVirtualTerminalProcessing)); ret != 0 {
		vtEnabled = true
	}
}
