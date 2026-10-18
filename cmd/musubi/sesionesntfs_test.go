package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// Estas pruebas miden NTFS —junctions y el bloqueo de un archivo abierto— y por eso corren sólo en
// Windows. El archivo NO lleva la plataforma en el nombre: compila en todos lados, y fuera de Windows
// cada prueba se saltea diciéndolo. Lo que hace las llamadas a Windows está en
// sesiones_ntfs_windows.go.

func soloEnNTFS(t *testing.T) {
	t.Helper()
	if runtime.GOOS != "windows" {
		t.Skipf("mide junctions y bloqueos de NTFS, que en %s no existen", runtime.GOOS)
	}
}

func abrirComoOtro(t *testing.T, ruta string, dejaBorrar bool) {
	t.Helper()
	cerrar, err := abrirComoOtroProceso(ruta, dejaBorrar)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(cerrar)
}

// Un `.jsonl` abierto por otro proceso. LO QUE PROTEGE A UNA SESIÓN ABIERTA NO ES EL BLOQUEO DE
// WINDOWS: Claude Code corre sobre libuv, que abre sus archivos CON FILE_SHARE_DELETE, y así el
// transcript se puede borrar con el proceso vivo. Lo que la protege es el registro de abiertas y la
// ventana de 15 minutos. El bloqueo sólo decide si un borrado puede quedar a medias, y el orden —el
// transcript primero— hace que no.
func TestSesionesJsonlAbiertoPorOtroEnWindows(t *testing.T) {
	soloEnNTFS(t)
	t.Run("sin compartir el borrado queda entera", func(t *testing.T) {
		config, projects := configDePrueba(t)
		sesionDePrueba(t, projects, "-p", idA, 10, 10, hace40Dias)
		jsonl := filepath.Join(projects, "-p", idA+".jsonl")
		abrirComoOtro(t, jsonl, false)
		if _, err := borrarSesion(config, "-p", idA); err == nil {
			t.Fatal("borrarSesion no avisó que no pudo borrar un .jsonl abierto sin FILE_SHARE_DELETE")
		}
		if !existe(jsonl) || !existe(filepath.Join(projects, "-p", idA, "subagents", "agent-a.jsonl")) {
			t.Fatal("la sesión quedó a medias: falta el .jsonl o su carpeta")
		}
	})

	t.Run("compartiendo el borrado el bloqueo no frena y el registro si", func(t *testing.T) {
		config, projects := configDePrueba(t)
		sesionDePrueba(t, projects, "-p", idA, 10, 10, hace40Dias)
		sesionDePrueba(t, projects, "-p", idB, 10, 10, hace40Dias)
		abrirComoOtro(t, filepath.Join(projects, "-p", idA+".jsonl"), true)
		abrirComoOtro(t, filepath.Join(projects, "-p", idB+".jsonl"), true)
		reg := filepath.Join(config, "sessions", "4242.json")
		escribir(t, reg, 0)
		if err := os.WriteFile(reg, []byte(`{"pid":4242,"sessionId":"`+idA+`","status":"idle"}`), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := borrarSesion(config, "-p", idA); err == nil || !existe(filepath.Join(projects, "-p", idA+".jsonl")) {
			t.Fatalf("se borró la sesión registrada como abierta (err=%v)", err)
		}
		// La de al lado está igual de abierta, pero sin registro y con 40 días: Windows la deja
		// borrar. Si esto falla, cambió el supuesto de arriba, no el comando.
		if _, err := borrarSesion(config, "-p", idB); err != nil {
			t.Fatalf("con FILE_SHARE_DELETE Windows no dejó borrar (err=%v): el supuesto cambió", err)
		}
	})
}

// La carpeta de proyecto misma es una junction a otro lado —en esta PC hay junctions de verdad, como
// D:\Proyectos—: ni se lista ni se borra nada del otro lado, aunque tenga la forma exacta de una sesión
// vieja.
func TestSesionesCarpetaDeProyectoQueEsJunction(t *testing.T) {
	soloEnNTFS(t)
	config, projects := configDePrueba(t)
	afuera := t.TempDir()
	sesionDePrueba(t, afuera, "real", idA, 10, 10, hace40Dias)
	if err := crearJunction(filepath.Join(afuera, "real"), filepath.Join(projects, "-j")); err != nil {
		t.Fatalf("no pude crear la junction, que no pide privilegios: %v", err)
	}
	jsonl := filepath.Join(afuera, "real", idA+".jsonl")
	if _, err := borrarSesion(config, "-j", idA); err == nil || !existe(jsonl) {
		t.Fatalf("borrarSesion borró a través de una carpeta de proyecto que es junction (err=%v)", err)
	}
	if code, salida := correrSesiones(t, "", "borrar", idA, "--si"); code == 0 || !existe(jsonl) {
		t.Fatalf("la CLI borró a través de la junction (salió %d):\n%s", code, salida)
	}
	if _, salida := correrSesiones(t, "", "--todas"); strings.Contains(salida, idA) {
		t.Errorf("el listado muestra una sesión que vive del otro lado de una junction:\n%s", salida)
	}
	if !existe(filepath.Join(afuera, "real", idA, "subagents", "agent-a.jsonl")) {
		t.Fatal("se borró la carpeta de la sesión del otro lado de la junction")
	}
}
