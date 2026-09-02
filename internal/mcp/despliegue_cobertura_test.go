package mcp

// Guardas del MAPA DE COBERTURA (deploy/verificar-cobertura.sh).
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// LA PREGUNTA QUE ESTE MAPA CONTESTA, Y POR QUÉ NO ALCANZABA CON LA DE A73
//
// A73 dejó `verificar-despliegue.sh`, que contesta «¿está la regla cargada?». El 2026-09-02, con
// las 35 reglas desplegadas y TODAS sus métricas presentes, se midió esto:
//
//	TemperaturaAlta        1 serie de 4 máquinas
//	CargaPorCoreAlta       2 de 4
//	ServicioReiniciandose  54 series, TODAS del servidor  → 0 de las 2 Windows
//	ServicioLento          1 serie                        → 1 servicio de 184
//
// Cada hueco tenía una razón buena —Windows no tiene load average, el SCM no expone reinicios, A2
// sigue abierto— y ninguna se podía leer desde ningún lado. La regla cargada, su métrica presente,
// y esa dimensión de esa máquina a ciegas. Verde por el motivo equivocado, otra vez.
//
// LA RAZÓN LA ESCRIBE LA REGLA, en su anotación `ausente_en:`, no el verificador. Es la misma
// forma que `# despliegue:` (A73): un catálogo de excepciones que vive en el verificador se
// desincroniza de las reglas y termina perdonando huecos que ya no corresponden.
// ────────────────────────────────────────────────────────────────────────────────────────────

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// selectoresDelVerificador saca el vocabulario del PROPIO script, y no de una copia acá.
//
// Si el test trajera su propia lista, agregar un selector al script sin tocar el test dejaría
// pasar anotaciones que el script no entiende — y el script las denuncia en producción, tarde.
// Peor al revés: sacar un selector del script y no del test haría pasar una anotación que ya no
// excusa nada, o sea un hueco real dibujado como decisión.
func selectoresDelVerificador(t *testing.T) []string {
	t.Helper()
	script := leerDeploy(t, "verificar-cobertura.sh")
	m := regexp.MustCompile(`SELECTORES\s*=\s*\(([^)]*)\)`).FindStringSubmatch(script)
	if m == nil {
		t.Fatal("no se encontró la tupla SELECTORES en verificar-cobertura.sh: el test perdió de " +
			"vista el vocabulario que valida, y un test que no encuentra nada pasa siempre")
	}
	var sels []string
	for _, s := range regexp.MustCompile(`'([^']+)'`).FindAllStringSubmatch(m[1], -1) {
		sels = append(sels, s[1])
	}
	if len(sels) < 2 {
		t.Fatalf("sólo se leyeron %d selectores del script; el patrón se rompió", len(sels))
	}
	return sels
}

// CADA HUECO DECLARADO USA UN SELECTOR QUE EL VERIFICADOR ENTIENDE, Y DA UNA RAZÓN.
//
// Un selector con typo no se ignora en silencio: el verificador lo denuncia y el hueco NO queda
// excusado. Pero eso se descubre corriendo el verificador contra producción — acá se descubre en
// la suite, que es donde cuesta un minuto en vez de una mañana.
//
// Y la razón importa tanto como el selector: `tier=B` a secas dice a quién no cubre y no dice por
// qué, así que el informe deja de ser accionable y a las dos semanas nadie lo lee.
//
// Sabotaje: poner `ausente_en: "so=windows — ..."` o quitarle la razón a una cláusula.
func TestCadaHuecoDeclaradoUsaUnSelectorQueElVerificadorEntiende(t *testing.T) {
	validos := selectoresDelVerificador(t)
	esValido := func(sel string) bool {
		for _, v := range validos {
			if v == sel || (strings.HasSuffix(v, "=") && strings.HasPrefix(sel, v) && len(sel) > len(v)) {
				return true
			}
		}
		return false
	}

	reAusente := regexp.MustCompile(`(?m)^\s*ausente_en:\s*"(.+)"\s*$`)
	total := 0
	for _, archivo := range []string{"musubi-alerts.yml", "musubi-alerts-flota.yml"} {
		texto := leerDeploy(t, archivo)
		// Se necesita el nombre de la alerta para que el error diga cuál es. Se recorre en orden.
		alerta := ""
		reAlerta := regexp.MustCompile(`^\s*-\s+alert:\s*(\S+)\s*$`)
		for _, l := range strings.Split(texto, "\n") {
			if m := reAlerta.FindStringSubmatch(l); m != nil {
				alerta = m[1]
				continue
			}
			m := reAusente.FindStringSubmatch(l)
			if m == nil {
				continue
			}
			total++
			clausulas := strings.Split(m[1], ";")
			validas := 0
			for _, c := range clausulas {
				if !strings.Contains(c, "—") {
					continue // prosa: la razón puede llevar punto y coma
				}
				partes := strings.SplitN(c, "—", 2)
				sel := strings.TrimSpace(partes[0])
				razon := strings.TrimSpace(partes[1])
				if !esValido(sel) {
					t.Errorf("%s declara el hueco con %q, que el verificador no entiende (válidos: %s).\n"+
						"El hueco NO queda excusado y se denuncia como hallazgo en producción.",
						alerta, sel, strings.Join(validos, ", "))
					continue
				}
				if len(razon) < 25 {
					t.Errorf("%s excusa %q con una razón de %d caracteres (%q).\n"+
						"El selector dice a QUIÉN no cubre; la razón dice POR QUÉ, y es lo único que "+
						"hace accionable el informe.", alerta, sel, len(razon), razon)
				}
				validas++
			}
			if validas == 0 {
				t.Errorf("%s tiene `ausente_en` sin ninguna cláusula válida: %q.\n"+
					"El formato es `<selector> — <razón>`, separando varias con `;`.", alerta, m[1])
			}
		}
	}
	if total == 0 {
		t.Fatal("ninguna regla declara `ausente_en`: o se borraron todas, o el patrón cambió y este " +
			"test dejó de mirar lo que dice mirar")
	}
}

// EL VERIFICADOR NO PUEDE PONERSE EN VERDE SOBRE EL CONJUNTO VACÍO.
//
// No es hipotético: pasó. Al juntar las veinticinco consultas en una sola conexión, las que
// llevaban espacios se partieron en varios argumentos, no volvió una sola serie, y el informe dijo
// «0/0 · toda ausencia de cobertura está declarada» — la frase más tranquilizadora posible sobre
// nada. Es la enfermedad que este verificador vino a cazar, cometida por el verificador.
//
// Sabotaje: borrar el bloque `if aplicables == 0` de verificar-cobertura.sh.
func TestElMapaDeCoberturaNoSePoneEnVerdeSinHaberMiradoNada(t *testing.T) {
	script := leerDeploy(t, "verificar-cobertura.sh")
	for _, guarda := range []string{"if not maquinas:", "aplicables == 0"} {
		if !strings.Contains(script, guarda) {
			t.Errorf("verificar-cobertura.sh perdió la guarda %q: sin ella, una consulta que no "+
				"devuelve nada se informa como cobertura perfecta, que es un falso verde esperando "+
				"su turno", guarda)
		}
	}
	// Y las dos tienen que salir con un código de ERROR, no con 0.
	i := strings.Index(script, "if not maquinas:")
	if i < 0 {
		return
	}
	cola := script[i:]
	if j := strings.Index(cola, "# ── El informe"); j > 0 {
		cola = cola[:j]
	}
	if strings.Count(cola, "sys.exit(2)") < 2 {
		t.Error("las guardas del conjunto vacío no cortan con un código de error: informar y seguir " +
			"deja el mismo verde que no tener la guarda")
	}
}

// LAS DOS COMPROBACIONES DE DESPLIEGUE SON HERMANAS Y NINGUNA REEMPLAZA A LA OTRA.
//
// `verificar-despliegue.sh` pregunta «¿está la regla?»; `verificar-cobertura.sh`, «¿esa regla
// vigila a esta máquina?». Con la primera sola, las 35 reglas cargadas se leen como 35 dimensiones
// vigiladas en las 4 máquinas — y son 13 de 19 en las Windows.
//
// Sabotaje: borrar cualquiera de los dos scripts.
func TestLasDosVerificacionesDeDespliegueExisten(t *testing.T) {
	for _, s := range []string{"verificar-despliegue.sh", "verificar-cobertura.sh"} {
		ruta := "../../deploy/" + s
		fi, err := os.Stat(ruta)
		if err != nil {
			t.Errorf("falta %s: %v", s, err)
			continue
		}
		if fi.Mode()&0o111 == 0 {
			t.Errorf("%s no es ejecutable: una comprobación que hay que invocar con `bash` de por "+
				"medio se corre menos, y la que no se corre no existe", s)
		}
	}
}
