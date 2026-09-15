package main

import (
	"os"
	"regexp"
	"strings"
	"testing"
	"unicode/utf16"
)

// TestLosSysoDeclaranLaVersionDeVERSION cierra A119: el ARTEFACTO que se compila tiene que decir
// la misma versión que la fuente, y hasta hoy nadie lo miraba.
//
// ════════════════════════════════════════════════════════════════════════════════════════════
// POR QUÉ NO ALCANZABA LA GUARDA QUE YA EXISTÍA
//
// `TestVersioninfoMatchesVERSION` (versioninfo_test.go) compara `VERSION` contra
// `versioninfo.json`. Las dos son FUENTES: ninguna es lo que termina adentro del `.exe`. Su propio
// comentario dice «versioninfo.json, de donde salen los .syso» —y nunca abre un `.syso`—, así que
// el eslabón que de verdad viaja al binario quedaba sin custodia.
//
// NO ES HIPOTÉTICO, Y LA MEDICIÓN ES DE HOY. El 2026-09-15, con `VERSION` en 0.141.0 y
// `versioninfo.json` recién bumpeado a 0.141.0.0, los dos `.syso` commiteados declaraban
// **0.139.1.0**: dos releases atrás. La guarda vieja estaba VERDE mientras el `.exe` de Windows
// mentía su versión en las Propiedades. El último commit que los había tocado era del 2026-09-08 y
// entre medio hubo dos bumps que nadie propagó.
//
// El cabo lo dice desde el 2026-09-08 y lo llama por su nombre: «fuente contra fuente, jamás toca
// el archivo que se compila».
//
// ════════════════════════════════════════════════════════════════════════════════════════════
// POR QUÉ SE LEE EN UTF-16 Y NO CON `strings`
//
// El recurso VERSIONINFO de Windows guarda sus cadenas en UTF-16LE. Un `strings` normal sobre el
// `.syso` devuelve VACÍO —lo medí, y casi lo leo como «el archivo no declara versión»—, que es el
// modo de falla clásico: un instrumento ciego produce una ausencia que se lee como un hecho.
// Acá se decodifica UTF-16 explícitamente.
//
// LA ASERCIÓN ES SOBRE LOS DOS ARCHIVOS, y no es redundante: `go generate` los produce con dos
// invocaciones distintas (`-64` y `-arm -64`), así que uno puede quedar viejo sin el otro. Con una
// sola arquitectura vigilada, el `.exe` de ARM podría mentir solo.
//
// EL SABOTAJE VA SOBRE EL .syso Y NO SOBRE versioninfo.json, Y COSTÓ DOS INTENTOS AVERIGUARLO.
//
// El primero cambiaba `versioninfo.json` a una versión vieja y dejaba la prueba EN VERDE. No era
// un hueco de la guarda: es que esta prueba NO ABRE ese archivo —lee `VERSION` y los `.syso`, y
// nada más—, así que tocarlo es un no-op. Un sabotaje inerte declarado como si funcionara es
// exactamente lo que el arnés advierte: enseña a confiar en una red que no está.
//
// El segundo intentó parchear los bytes UTF-16 del `.syso` con `perl` y no matcheó nada; el arnés
// lo dijo por su nombre («el sabotaje NO cambió el archivo») en vez de dar un verde silencioso.
//
// El que sí muerde es el de abajo, y es además el defecto REAL que se quiere atrapar: regenerar el
// recurso desde un `versioninfo.json` desactualizado. Medido: cae en el `t.Errorf` de «declara la
// versión … y VERSION dice …», que es la aserción de esta prueba y no otra. (Acá decía «la línea
// 75» y para cuando terminé de escribir este comentario la aserción estaba en la 89: un número de
// línea es un derivado que nace rancio, así que se nombra la aserción y no su posición.)
//
// Sabotaje que la hace fallar: regenerar rsrc_windows_amd64.syso desde un versioninfo.json con la
// versión vieja (0.139.1.0), dejando VERSION en 0.141.0. El comando exacto:
//
//	sed 's/"FileVersion": "0.141.0.0"/"FileVersion": "0.139.1.0"/; s/"Minor": 141/"Minor": 139/g' \
//	    versioninfo.json > /tmp/vi-viejo.json
//	go run github.com/josephspurrier/goversioninfo/cmd/goversioninfo@v1.4.0 \
//	    -64 -o rsrc_windows_amd64.syso /tmp/vi-viejo.json
//
// arnes: no_mecanizable="el sabotaje que muerde REGENERA un binario (.syso) con goversioninfo desde un versioninfo.json alterado: no es una sustitución de texto en un archivo del repo, que es lo único que este arnés sabe aplicar. Se corrió a mano el 2026-09-15 y quedó en ROJO por la aserción de la línea 75 — el comando exacto está en el comentario de arriba."
func TestLosSysoDeclaranLaVersionDeVERSION(t *testing.T) {
	crudo, err := os.ReadFile("../../VERSION")
	if err != nil {
		t.Fatalf("no se pudo leer VERSION: %v", err)
	}
	version := strings.TrimSpace(string(crudo))
	if strings.Count(version, ".") != 2 {
		t.Fatalf("VERSION debe tener el formato X.Y.Z, es %q", version)
	}
	// El recurso de Windows lleva CUATRO componentes: X.Y.Z.0, igual que StringFileInfo.
	quiero := version + ".0"

	for _, syso := range []string{"rsrc_windows_amd64.syso", "rsrc_windows_arm64.syso"} {
		t.Run(syso, func(t *testing.T) {
			vistas := versionesEnSyso(t, syso)
			if len(vistas) == 0 {
				t.Fatalf("%s no declara NINGUNA versión X.Y.Z.W.\n"+
					"  O el archivo no es un recurso VERSIONINFO válido, o la lectura UTF-16 dejó de\n"+
					"  encontrarla. Cero versiones NO es «coincide»: es que esta guarda no está midiendo.",
					syso)
			}
			// Se exige que TODAS las que aparecen sean la esperada, no que «alguna» lo sea: el
			// recurso declara FileVersion y ProductVersion, y aceptar que una sola coincida dejaría
			// pasar el caso en que se bumpea media estructura.
			for _, v := range vistas {
				if v != quiero {
					t.Errorf("%s declara la versión %q y VERSION dice %q (esperado %q en el recurso).\n"+
						"  El .exe de Windows va a mostrar %q en sus Propiedades: el artefacto que se\n"+
						"  compila no corresponde a la fuente. Regeneralos desde cmd/musubi/ con:\n"+
						"      go generate ./...\n"+
						"  y commiteá los dos .syso en el MISMO commit que el bump, como exige\n"+
						"  versioninfo_gen.go («no toques los .syso a mano»).",
						syso, v, version, quiero, v)
				}
			}
		})
	}
}

// versionesEnSyso devuelve las versiones X.Y.Z.W que el recurso declara, decodificando UTF-16LE.
//
// Devuelve el CONJUNTO ordenado por aparición y sin repetir: un recurso sano declara la misma
// cadena en FileVersion y ProductVersion, así que lo normal es una sola.
func versionesEnSyso(t *testing.T, ruta string) []string {
	t.Helper()
	b, err := os.ReadFile(ruta)
	if err != nil {
		t.Fatalf("no se pudo leer %s: %v", ruta, err)
	}
	// UTF-16LE: pares de bytes. Se decodifica entero y se busca sobre el texto resultante, que es
	// lo que hace legible la cadena de versión (un `strings` ASCII devuelve vacío acá).
	u16 := make([]uint16, 0, len(b)/2)
	for i := 0; i+1 < len(b); i += 2 {
		u16 = append(u16, uint16(b[i])|uint16(b[i+1])<<8)
	}
	texto := string(utf16.Decode(u16))

	re := regexp.MustCompile(`\d+\.\d+\.\d+\.\d+`)
	var out []string
	visto := map[string]bool{}
	for _, m := range re.FindAllString(texto, -1) {
		if !visto[m] {
			visto[m] = true
			out = append(out, m)
		}
	}
	return out
}
