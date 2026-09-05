package main

// rustdesk_ruta_test.go custodia el arreglo del fallo más caro del track de pantalla: el agente
// buscaba `rustdesk` en el PATH, Windows no lo pone ahí, y una máquina CON RustDesk instalado se
// veía exactamente igual que una sin él.
//
// Lo que se prueba no es «encuentra el binario» —eso depende de la máquina— sino las tres cosas
// que hacían que el fallo fuera invisible:
//
//  1. La lista de lugares de WINDOWS es correcta, mirada desde Linux (que es donde corre la suite).
//  2. Un override mal puesto FALLA en vez de caer de vuelta a la búsqueda.
//  3. Cuando no aparece, el error DICE DÓNDE MIRÓ.

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestEnWindowsSeBuscaDondeLoDejaElInstaladorOficial es la prueba que no existía y por la que el
// fallo vivió un track entero.
//
// `C:\Program Files\RustDesk\rustdesk.exe` es la ruta REAL, verificada en las dos máquinas
// Windows de la flota. Si alguien saca esa entrada de la lista, el plano de pantalla vuelve a
// quedar mudo en la única plataforma donde hace falta.
//
// Sabotaje que la hace fallar: borrar `os.Getenv("ProgramFiles")` de candidatosRustdeskPara.
func TestEnWindowsSeBuscaDondeLoDejaElInstaladorOficial(t *testing.T) {
	// Cada variable se prueba SOLA. La primera versión de esta prueba las ponía todas juntas con
	// los valores reales de una máquina —donde ProgramW6432 y ProgramFiles valen lo mismo— y
	// entonces borrar una de las dos del código no rompía nada: la otra tapaba el agujero. Una
	// prueba que pasa con el código saboteado no es una prueba.
	casos := []struct {
		variable string
		valor    string
		espera   string
		porque   string
	}{
		{"ProgramFiles", `C:\Program Files`, `c:/program files/rustdesk/rustdesk.exe`,
			"la instalación con administrador, que es la que hay en las dos máquinas Windows de la flota"},
		{"ProgramW6432", `C:\Program Files`, `c:/program files/rustdesk/rustdesk.exe`,
			"un agente de 32 bits ve el directorio de 64 sólo por esta variable"},
		{"ProgramFiles(x86)", `C:\Program Files (x86)`, `c:/program files (x86)/rustdesk/rustdesk.exe`,
			"instalaciones viejas de 32 bits"},
		{"LOCALAPPDATA", `C:\Users\gio\AppData\Local`, `c:/users/gio/appdata/local/programs/rustdesk/rustdesk.exe`,
			"la instalación por-usuario, que no pide administrador"},
		{"APPDATA", `C:\Users\gio\AppData\Roaming`, `c:/users/gio/appdata/roaming/rustdesk/rustdesk.exe`,
			"algunas versiones portables"},
	}
	for _, c := range casos {
		t.Run(c.variable, func(t *testing.T) {
			// SÓLO esta variable tiene valor: si el código deja de leerla, no hay otra que la tape.
			rutas := candidatosRustdeskPara("windows", func(k string) string {
				if k == c.variable {
					return c.valor
				}
				return ""
			})
			if !strings.Contains(normalizarRutas(rutas), c.espera) {
				t.Errorf("con %s=%s no se busca en %s — %s.\nLugares mirados: %v",
					c.variable, c.valor, c.espera, c.porque, rutas)
			}
		})
	}

	// Y el ORDEN: ProgramW6432 antes que (x86). Si el agente corriera como proceso de 32 bits,
	// %ProgramFiles% apunta al directorio de 32 y el cliente de 64 quedaría sin encontrar.
	entorno := map[string]string{
		"ProgramW6432":      `C:\Program Files`,
		"ProgramFiles(x86)": `C:\Program Files (x86)`,
	}
	normal := normalizarRutas(candidatosRustdeskPara("windows", func(k string) string { return entorno[k] }))
	i64 := strings.Index(normal, "c:/program files/rustdesk")
	i86 := strings.Index(normal, "c:/program files (x86)/rustdesk")
	if i64 < 0 || i86 < 0 || i64 > i86 {
		t.Errorf("la ruta de 64 bits no se mira antes que la de 32 (64=%d, 32=%d): un agente de 32 bits encontraría el binario equivocado o ninguno", i64, i86)
	}
}

// normalizarRutas deja las rutas comparables desde Linux: la suite corre ahí, filepath.Join arma
// con "/" y lo que importa es QUÉ directorios se miran, no cómo se escriben.
func normalizarRutas(rutas []string) string {
	return strings.ToLower(strings.ReplaceAll(strings.Join(rutas, "\n"), `\`, "/"))
}

// TestUnOverrideQueApuntaANadaFallaEnVezDeSeguirBuscando.
//
// Es deliberado y va contra el instinto de «ser tolerante»: si MUSUBI_RUSTDESK_BIN apunta a algo
// que no existe y el código sigue buscando, la variable no se puede depurar — alguien la pone
// mal, la pantalla anda igual por otro camino, y el día que el otro camino no está nadie entiende
// por qué. Un override que no se respeta es peor que no tenerlo.
//
// Sabotaje que la hace fallar: reemplazar el return por un `// seguir buscando`.
func TestUnOverrideQueApuntaANadaFallaEnVezDeSeguirBuscando(t *testing.T) {
	t.Setenv("MUSUBI_RUSTDESK_BIN", filepath.Join(t.TempDir(), "no-existe"))
	_, err := rutaRustdesk()
	if err == nil {
		t.Fatal("un MUSUBI_RUSTDESK_BIN inválido no falló: el override quedaría mudo y sin forma de depurarlo")
	}
	if !strings.Contains(err.Error(), "MUSUBI_RUSTDESK_BIN") {
		t.Errorf("el error no nombra la variable que está mal puesta: %v", err)
	}
	// Y NO puede ser el error de «no está instalado»: son dos problemas distintos con dos
	// arreglos distintos, y confundirlos es el bug que este archivo vino a cerrar.
	if errors.Is(err, errSinRustdesk) {
		t.Error("un override mal puesto se reporta como «RustDesk no está instalado»: manda a instalar algo que ya está")
	}
}

// TestUnOverrideValidoSeUsaTalCual — la contraparte: si apunta a algo que existe, se usa.
func TestUnOverrideValidoSeUsaTalCual(t *testing.T) {
	falso := filepath.Join(t.TempDir(), "rustdesk-de-mentira")
	if err := os.WriteFile(falso, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("MUSUBI_RUSTDESK_BIN", falso)
	got, err := rutaRustdesk()
	if err != nil {
		t.Fatalf("un override válido falló: %v", err)
	}
	if got != falso {
		t.Errorf("se usó %q en vez del override %q", got, falso)
	}
}

// TestCuandoNoApareceElErrorDiceDondeSeBusco.
//
// Un «no se encontró RustDesk» a secas obliga a adivinar. La lista de lugares mirados convierte
// el diagnóstico en una comparación: está acá, no está en ninguno de esos, ya sé qué poner en
// MUSUBI_RUSTDESK_BIN.
//
// Sabotaje que la hace fallar: volver el error a un fmt.Errorf sin los candidatos.
func TestCuandoNoApareceElErrorDiceDondeSeBusco(t *testing.T) {
	// Se prueba el mensaje, no la máquina. La versión anterior vaciaba el PATH y se salteaba
	// (t.Skip) en cualquier equipo que tuviera RustDesk instalado — como el que escribió esto,
	// donde /usr/bin/rustdesk existe. Se salteó, pasó en verde, y no protegía nada.
	candidatos := []string{`C:\Program Files\RustDesk\rustdesk.exe`, `C:\Users\gio\AppData\Local\Programs\RustDesk\rustdesk.exe`}
	err := errorSinRustdesk(candidatos)

	if !errors.Is(err, errSinRustdesk) {
		t.Errorf("la ausencia no se reporta como errSinRustdesk, así que arriba no se puede distinguir de un fallo real: %v", err)
	}
	for _, c := range candidatos {
		if !strings.Contains(err.Error(), c) {
			t.Errorf("el error no dice que miró en %s: obliga a adivinar dónde poner el binario.\nMensaje: %v", c, err)
		}
	}
	if !strings.Contains(err.Error(), "MUSUBI_RUSTDESK_BIN") {
		t.Errorf("el error no dice cuál es la salida de emergencia: %v", err)
	}
}

// TestElDobleDePruebaLeGanaATodo mantiene viva la costura que usa pantalla_test.go.
func TestElDobleDePruebaLeGanaATodo(t *testing.T) {
	anterior := binarioRustdesk
	binarioRustdesk = "/un/doble/cualquiera"
	t.Cleanup(func() { binarioRustdesk = anterior })
	t.Setenv("MUSUBI_RUSTDESK_BIN", "/otra/cosa")

	got, err := rutaRustdesk()
	if err != nil || got != "/un/doble/cualquiera" {
		t.Fatalf("el doble de prueba dejó de tener prioridad (got=%q err=%v): las pruebas de pantalla ejecutarían un binario real", got, err)
	}
}

// TestEnLinuxYMacSeMiranLasRutasDeSusInstaladores — el resto de las plataformas, más barato pero
// por la misma razón: que la lista no quede vacía sin que nadie se entere.
func TestEnLinuxYMacSeMiranLasRutasDeSusInstaladores(t *testing.T) {
	casos := map[string]string{
		"linux":  "/usr/bin/rustdesk",
		"darwin": "/Applications/RustDesk.app/Contents/MacOS/rustdesk",
	}
	for goos, quiero := range casos {
		rutas := candidatosRustdeskPara(goos, func(string) string { return "" })
		if !strings.Contains(strings.Join(rutas, "\n"), quiero) {
			t.Errorf("en %s no se busca en %s: %v", goos, quiero, rutas)
		}
	}
}
