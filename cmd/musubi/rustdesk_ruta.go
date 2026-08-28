package main

// rustdesk_ruta.go resuelve UNA pregunta: dónde está el cliente RustDesk en ESTA máquina.
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// POR QUÉ ESTE ARCHIVO EXISTE
//
// Antes, el agente ejecutaba `rustdesk` a secas y dejaba que el sistema lo buscara en el PATH.
// En Linux eso alcanza. En Windows NO: el instalador oficial deja el binario en
// `C:\Program Files\RustDesk\rustdesk.exe` y **no toca el PATH**. Resultado, verificado en la
// flota real: dos máquinas con RustDesk instalado y CORRIENDO, y el inventario sin un solo
// `rustdesk_id` — porque `idRustdeskLocal` no lo encontraba y devolvía "" en silencio, que el
// código de arriba lee como «esta máquina no tiene pantalla configurada todavía».
//
// Ese es el fallo peor de todo el track: no es que la pantalla no ande, es que la ausencia de
// pantalla se ve EXACTAMENTE IGUAL esté RustDesk instalado o no. Por eso acá la búsqueda es
// explícita, la lista de lugares está escrita, y cuando no aparece el error DICE DÓNDE MIRÓ.
// ────────────────────────────────────────────────────────────────────────────────────────────

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// binarioRustdesk fuerza la ruta del cliente. Vacío —el default— significa «descubrila».
//
// Sigue siendo `var` por la misma razón de siempre: las pruebas lo apuntan a un doble, y así la
// integración con el binario real es lo único del track que necesita una máquina de verdad.
var binarioRustdesk = ""

// errSinRustdesk distingue «no está instalado» de «está y falló». Son dos cosas distintas y
// mezclarlas fue el bug: una es una máquina sin plano visual, la otra es un plano visual roto.
var errSinRustdesk = errors.New("no se encontró el cliente RustDesk en esta máquina")

// rutaRustdesk devuelve el ejecutable del cliente, o errSinRustdesk envuelto con los lugares
// donde miró.
//
// El orden importa y es de más explícito a menos:
//
//  1. binarioRustdesk (doble de prueba).
//  2. MUSUBI_RUSTDESK_BIN — la salida de emergencia para una instalación rara. Si apunta a algo
//     que no existe es un ERROR, no un motivo para seguir buscando: un override equivocado que
//     cae de vuelta al descubrimiento es un override que no se puede depurar.
//  3. El PATH.
//  4. Los lugares donde el instalador oficial lo deja, por sistema operativo.
func rutaRustdesk() (string, error) {
	if binarioRustdesk != "" {
		return binarioRustdesk, nil
	}
	if forzado := strings.TrimSpace(os.Getenv("MUSUBI_RUSTDESK_BIN")); forzado != "" {
		if !esEjecutable(forzado) {
			return "", fmt.Errorf("MUSUBI_RUSTDESK_BIN apunta a %q y ahí no hay un ejecutable: corregilo o sacá la variable (no se busca en otro lado a propósito, para que un override mal puesto se vea)", forzado)
		}
		return forzado, nil
	}
	if p, err := exec.LookPath(nombreBinarioRustdesk()); err == nil {
		return p, nil
	}
	candidatos := candidatosRustdesk()
	for _, c := range candidatos {
		if esEjecutable(c) {
			return c, nil
		}
	}
	return "", errorSinRustdesk(candidatos)
}

// errorSinRustdesk arma el mensaje de «no está», con los lugares donde se miró.
//
// Está aparte de rutaRustdesk por una razón de prueba concreta: rutaRustdesk depende de qué haya
// instalado en la máquina que corre la suite, así que una prueba que intente forzar la ausencia
// termina en un t.Skip en cualquier máquina que TENGA RustDesk — y una prueba que se saltea es
// una prueba que no protege nada. Con la construcción del mensaje separada, el contenido del
// error se verifica siempre, esté RustDesk instalado o no.
func errorSinRustdesk(candidatos []string) error {
	return fmt.Errorf("%w: se buscó en el PATH y en %s. Si está en otro lado, poné MUSUBI_RUSTDESK_BIN con la ruta completa",
		errSinRustdesk, strings.Join(candidatos, ", "))
}

func nombreBinarioRustdesk() string {
	if runtime.GOOS == "windows" {
		return "rustdesk.exe"
	}
	return "rustdesk"
}

// candidatosRustdesk son los lugares donde el instalador OFICIAL deja el cliente.
//
// No es una lista de adivinanzas: cada entrada corresponde a un instalador publicado. Las de
// Windows llevan variables de entorno porque `C:\Program Files` no está en C: en todas las
// máquinas ni se llama igual en todos los idiomas de Windows.
func candidatosRustdesk() []string {
	return candidatosRustdeskPara(runtime.GOOS, os.Getenv)
}

// candidatosRustdeskPara es la de arriba con el sistema y el entorno como PARÁMETROS.
//
// No es una indirección por gusto: el fallo que este archivo arregla es EXCLUSIVAMENTE de
// Windows, y la suite corre en Linux. Sin esta firma, la lista de rutas de Windows —la única
// parte que importaba— sería justo la que ninguna prueba puede mirar.
func candidatosRustdeskPara(goos string, env func(string) string) []string {
	switch goos {
	case "windows":
		var rs []string
		// ProgramW6432 va PRIMERO: si el agente corriera como proceso de 32 bits, %ProgramFiles%
		// apunta al directorio (x86) y el cliente de 64 no estaría ahí.
		for _, base := range []string{
			env("ProgramW6432"),
			env("ProgramFiles"),
			env("ProgramFiles(x86)"),
		} {
			if base != "" {
				rs = append(rs, filepath.Join(base, "RustDesk", "rustdesk.exe"))
			}
		}
		// La instalación por-usuario, que no pide administrador y es la que elige mucha gente.
		if la := env("LOCALAPPDATA"); la != "" {
			rs = append(rs, filepath.Join(la, "Programs", "RustDesk", "rustdesk.exe"))
		}
		if ad := env("APPDATA"); ad != "" {
			rs = append(rs, filepath.Join(ad, "RustDesk", "rustdesk.exe"))
		}
		return rs
	case "darwin":
		return []string{"/Applications/RustDesk.app/Contents/MacOS/rustdesk"}
	default:
		return []string{
			"/usr/bin/rustdesk",
			"/usr/local/bin/rustdesk",
			"/opt/rustdesk/rustdesk",
			// Flatpak exporta un nombre distinto al del binario.
			"/var/lib/flatpak/exports/bin/com.rustdesk.RustDesk",
		}
	}
}

// esEjecutable no mira el bit de ejecución en Windows —ahí no significa nada— y sí en el resto.
func esEjecutable(p string) bool {
	fi, err := os.Stat(p)
	if err != nil || fi.IsDir() {
		return false
	}
	if runtime.GOOS == "windows" {
		return true
	}
	return fi.Mode().Perm()&0o111 != 0
}
