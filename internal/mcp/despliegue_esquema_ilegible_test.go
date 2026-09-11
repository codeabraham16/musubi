package mcp

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"musubi/internal/guiones"
)

// NO PODER LEER EL ESQUEMA NO PUEDE TIRAR ABAJO UN DESPLIEGUE — Y TIRARLO REVIERTE LA BASE.
//
// ════════════════════════════════════════════════════════════════════════════════════════════
// EL DEFECTO, MEDIDO Y NO RAZONADO (2026-09-11)
//
// La lectura del esquema en `redesplegar-cerebro.sh` decía:
//
//	ESQUEMA="$(python3 -c "…PRAGMA user_version…" "$BASE" 2>/dev/null || echo 0)"
//
// Cualquier motivo por el que esa lectura fallara —python3 ausente, el módulo `sqlite3` sin
// compilar, la base bloqueada por otro proceso, una ruta equivocada— dejaba `ESQUEMA=0`. Y el
// `elif` de abajo compara contra lo que el binario declara: `0 != 54` cae derecho en
// `volver_atras`.
//
// O SEA QUE «NO PUDE MEDIR» ENTRABA POR LA PUERTA DE «MEDÍ Y DA DISTINTO», y el mensaje acusaba la
// causa equivocada —«la migración no llegó»— cuando la migración había llegado perfecto y lo que
// falló fue el instrumento. Corrido contra el bloque viejo, con un python3 que sale 1 y una base
// SANA en esquema 7:
//
//	>>> VOLVER_ATRAS: la migración no llegó: el esquema quedó en 0 y este binario apunta a 7
//
// Y NO ES UN AVISO FEO: `volver_atras` hace `cp -a "$RESPALDO" "$BASE"`, borra el WAL y el SHM, y
// sale en 1. Un cero que significa «no sé» disparando una RESTAURACIÓN DE DATOS sobre el cerebro
// central. En una máquina sin python3, además, sería permanente: todo redespliegue volvería atrás.
//
// LA REGLA CORRECTA YA ESTABA ESCRITA EN EL MISMO BLOQUE, PARA EL HERMANO. Cuando el que no sabe
// decir el esquema es el BINARIO (`version --esquema` vacío), el guion avisa y sigue, con el
// motivo escrito: «no es motivo para tirar abajo un despliegue». Lo mismo vale cuando el que no
// puede contestar es el disco. La regla estaba en un lado y no en el otro — el defecto dominante
// de este repo.
//
// ════════════════════════════════════════════════════════════════════════════════════════════
// POR QUÉ ESTA GUARDA CORRE EL GUION EN VEZ DE LEERLO
//
// Un grep por `|| echo 0` lo satisface cualquier reescritura: `|| echo -1`, `ESQUEMA=${X:-0}`, un
// `set +e` con un `[[ -z ]]` que asigna cero más abajo. Este repo ya tuvo siete guardas en verde
// satisfechas por un comentario o por la forma del bug en vez de por el bug. Lo que decide acá no
// es cómo está escrita la línea sino QUÉ HACE EL GUION cuando la lectura falla, así que el arnés
// EXTRAE el bloque del archivo real —no una copia a mano, que sería el mismo derivado tipeado que
// este track viene matando— y lo CORRE con un `python3` de mentira.
//
// Los cuatro casos cubren las dos direcciones. Sin el control positivo (el esquema que de verdad
// no coincide y SÍ tiene que volver atrás), una guarda que impidiera volver atrás SIEMPRE pasaría
// por buena, y eso sería mucho peor que el defecto que cierra.
func TestNoPoderLeerElEsquemaNoTiraAbajoElDespliegue(t *testing.T) {
	guiones.Exigir(t, "corre el tramo de verificación de migración de deploy/redesplegar-cerebro.sh, que decide si se RESTAURA la base del cerebro central sobre un servidor Linux", "bash")

	const rel = "redesplegar-cerebro.sh"
	guion := leerGuionDeDespliegue(t, rel)
	bloque := bloqueEntreMarcas(t, guion, rel,
		`ESPERADO="$("$DESTINO" version --esquema`,
		"# Que responda de verdad")

	casos := []struct {
		nombre      string
		python3     string // cuerpo del stub
		declaraBin  string // lo que contesta `version --esquema`
		vuelveAtras bool
		porque      string
	}{
		{
			nombre:      "la lectura del esquema FALLA",
			python3:     "exit 1\n",
			declaraBin:  "7",
			vuelveAtras: false,
			porque: "no se pudo LEER el esquema y el guion RESTAURÓ LA BASE igual. Eso es «no pude medir» " +
				"entrando por la puerta de «medí y da distinto», y la consecuencia no es un aviso feo: " +
				"`volver_atras` hace `cp -a $RESPALDO $BASE` sobre el cerebro central",
		},
		{
			nombre:      "la lectura contesta algo que no es un numero",
			python3:     "echo 'no-es-un-numero'\n",
			declaraBin:  "7",
			vuelveAtras: false,
			porque: "la lectura del esquema devolvió basura y el guion la trató como un esquema. El " +
				"productor es un `python3 -c` y el consumidor una comparación de shell: sin nadie en el " +
				"medio, una salida rara entra como si fuera una medición",
		},
		{
			nombre:      "CONTROL POSITIVO: el esquema NO coincide de verdad",
			python3:     "echo 7\n",
			declaraBin:  "9",
			vuelveAtras: true,
			porque: "el esquema en disco es 7, el binario apunta a 9 y el guion NO volvió atrás. La " +
				"migración no llegó y el despliegue siguió: esta guarda impide volver atrás SIEMPRE, que " +
				"es peor que el defecto que vino a cerrar",
		},
		{
			nombre:      "CONTROL: el esquema coincide",
			python3:     "echo 7\n",
			declaraBin:  "7",
			vuelveAtras: false,
			porque:      "el esquema coincidía con el que el binario declara y el guion volvió atrás igual",
		},
	}

	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			dir := t.TempDir()
			stubs := filepath.Join(dir, "stubs")
			if err := os.MkdirAll(stubs, 0o755); err != nil {
				t.Fatal(err)
			}
			escribirStub(t, stubs, "python3", c.python3)

			destino := filepath.Join(dir, "musubi")
			escribirStub(t, dir, "musubi", "echo "+shQuote(c.declaraBin)+"\n")

			marca := filepath.Join(dir, "volvio-atras")
			arnes := []string{
				"#!/usr/bin/env bash",
				"set -uo pipefail",
				"export PATH=" + shQuote(stubs) + `:"$PATH"`,
				funcionesDeSalida(t, guion, rel),
				// `volver_atras` se reemplaza a propósito: la de verdad para servicios con systemctl y
				// copia archivos como root. Lo que esta prueba mide es SI SE LLAMA, no qué hace adentro.
				"volver_atras(){ : > " + shQuote(marca) + "; exit 1; }",
				"DESTINO=" + shQuote(destino),
				"BASE=" + shQuote(filepath.Join(dir, "memory.db")),
				"VERSION_BASE=de-prueba",
				bloque,
			}
			arnesP := filepath.Join(dir, "arnes.sh")
			if err := os.WriteFile(arnesP, []byte(strings.Join(arnes, "\n")+"\n"), 0o755); err != nil {
				t.Fatal(err)
			}

			salida, _ := exec.Command("bash", arnesP).CombinedOutput()
			_, err := os.Stat(marca)
			volvio := err == nil

			if volvio != c.vuelveAtras {
				t.Fatalf("%s\n  esperaba vuelta atrás=%v, fue=%v\n  salida del guion:\n%s",
					c.porque, c.vuelveAtras, volvio, salida)
			}

			// CONTROL DE QUE EL ARNÉS MIDIÓ ALGO. Un bloque que no imprimiera nada daría «no volvió
			// atrás» en los tres casos que lo esperan, y pasaría por bueno sin haber ejecutado una
			// línea. Los cuatro caminos del `if` terminan en `aviso`, `volver_atras` u `ok`.
			if !volvio && len(strings.TrimSpace(string(salida))) == 0 {
				t.Fatalf("el bloque no imprimió NADA: no volvió atrás porque no se ejecutó, no porque " +
					"decidiera no hacerlo. Un arnés mudo no mide nada")
			}
		})
	}
}
