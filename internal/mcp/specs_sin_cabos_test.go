package mcp

// Guarda del track «Control de flota»: NINGÚN cabo suelto sin registro.
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// POR QUÉ ESTO ES UNA PRUEBA Y NO UNA COSTUMBRE
//
// `specs/control-de-flota/ABIERTO.md` dice, en su propia sección «Cómo se usa este archivo»:
// «Un `## Lo que queda fuera` en un spec que no aparezca acá es un cabo suelto de verdad».
//
// Esa regla se cumplió a mano durante todo el track, y a mano se rompió: un barrido encontró
// NUEVE ítems declarados fuera de alcance que ya se habían hecho en slices posteriores —specs
// afirmando que algo «no está» cuando estaba— y dos que nunca tuvieron número de registro.
// Un spec que miente sobre lo que falta es peor que un pendiente: quien lo lee aprende algo falso
// y decide con eso.
//
// Así que la regla deja de depender de que alguien se acuerde. Cada ítem de un `## Lo que queda
// fuera` tiene que declarar UNA de estas cosas:
//
//   - su número de registro (**A17**, **B4**) — está anotado y tiene dueño;
//   - en qué slice se HIZO o se DESCARTÓ — ya no falta;
//   - «por diseño» / «despliegue» — nunca fue código de este track;
//   - «Cero dependencias nuevas» — es la coletilla de disciplina, no un pendiente.
//
// Si agregás un cabo nuevo sin ninguna de esas marcas, esta prueba falla y te manda a ABIERTO.md.
// Ese es exactamente el punto.
// ────────────────────────────────────────────────────────────────────────────────────────────

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var (
	encabezadoFuera = regexp.MustCompile(`(?i)^#+\s*lo que queda fuera`)
	// Un ítem de primer nivel: `- **algo**`. Las continuaciones van indentadas y no cuentan.
	itemDeCabo = regexp.MustCompile(`^- \*\*`)
	// Las marcas que dan por CUBIERTO un cabo. `S7c` y `S5b` incluidos: el sufijo es parte del
	// nombre del slice, no ruido.
	tieneCasa = regexp.MustCompile(`HECH|DESCARTAD|\*\*A\d+|\*\*B\d+|\*\*S\d+[a-z]?\*\*|por diseño|Cero dependencias|despliegue`)
)

func TestNingunCaboDeFlotaSeQuedaSinRegistro(t *testing.T) {
	specs, err := filepath.Glob(filepath.Join("..", "..", "specs", "flota-*", "tasks.md"))
	if err != nil {
		t.Fatal(err)
	}
	// Si el glob deja de encontrar los specs (se movieron, se renombraron), la prueba pasaría
	// vacía y en verde — el modo de fallo más peligroso que puede tener un barrido.
	if len(specs) < 10 {
		t.Fatalf("sólo se encontraron %d specs de flota; el barrido no está mirando donde cree", len(specs))
	}

	huerfanos := 0
	for _, ruta := range specs {
		crudo, err := os.ReadFile(ruta)
		if err != nil {
			t.Fatal(err)
		}
		dentro := false
		for n, linea := range strings.Split(string(crudo), "\n") {
			if strings.HasPrefix(linea, "#") {
				dentro = encabezadoFuera.MatchString(linea)
				continue
			}
			if !dentro || !itemDeCabo.MatchString(linea) {
				continue
			}
			if tieneCasa.MatchString(linea) {
				continue
			}
			huerfanos++
			corto := linea
			if len(corto) > 90 {
				corto = corto[:90] + "…"
			}
			t.Errorf("%s:%d — cabo sin registro:\n    %s\n  Anotalo en specs/control-de-flota/ABIERTO.md (tabla 1 con slice, o tabla 2 con la condición bajo la que se revisa) y nombrá acá su número.",
				ruta, n+1, corto)
		}
	}
	if huerfanos == 0 {
		t.Logf("%d specs de flota barridos, cero cabos sin registro", len(specs))
	}
}

// El registro tiene que seguir EXISTIENDO y conservar sus dos tablas. Si alguien lo borra o lo
// vacía, la prueba de arriba seguiría en verde (los specs no cambiaron) mientras el archivo al
// que mandan sus mensajes deja de decir nada.
func TestElRegistroDeAbiertosSigueEnPie(t *testing.T) {
	crudo, err := os.ReadFile(filepath.Join("..", "..", "specs", "control-de-flota", "ABIERTO.md"))
	if err != nil {
		t.Fatalf("no se pudo leer el registro de abiertos: %v", err)
	}
	texto := string(crudo)
	for _, quiero := range []struct{ frag, porque string }{
		{"## 1 · Con slice asignado", "la tabla de lo que SÍ se va a hacer"},
		{"## 2 · Decisiones de NO hacer", "la tabla de lo declarado fuera, con su condición de revisión"},
		{"| A", "no queda ni un cabo con dueño: o se terminó todo, o alguien vació la tabla"},
		{"| B", "no queda ni una decisión de no-hacer: sospechoso"},
	} {
		if !strings.Contains(texto, quiero.frag) {
			t.Errorf("ABIERTO.md perdió %q — %s", quiero.frag, quiero.porque)
		}
	}
}
