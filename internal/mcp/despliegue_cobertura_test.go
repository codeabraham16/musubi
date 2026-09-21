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
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"musubi/internal/guiones"
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
// arnes: archivo="deploy/musubi-alerts-flota.yml"
// arnes: de="          ausente_en: \"os=windows — el load average es un concepto de UNIX y ahí no existe; emitir un 0 lo haría indistinguible de una máquina ociosa.\""
// arnes: a="          ausente_en: \"so=windows — el load average es un concepto de UNIX y ahí no existe; emitir un 0 lo haría indistinguible de una máquina ociosa.\""
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
// arnes: archivo="deploy/verificar-cobertura.sh"
// arnes: de="if aplicables == 0:"
// arnes: a="if aplicables < 0:"
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
// EL MODO SE LE PREGUNTA A GIT, NO AL DISCO (medido el 2026-09-19). La versión anterior leía el
// bit con `os.Stat`, y eso deja pasar el único estado que importa: el índice en 100644 con el disco
// parchado a mano en 775. Reproducido acá —`git update-index --chmod=-x` sobre los dos guiones— la
// guarda daba VERDE, y `git checkout-index` materializaba los dos en 664, que es lo que recibe
// cualquier clone. No es una hipótesis: `deploy/musubi-backup.sh` vivió en 100644 desde el
// 2026-07-08 y su unidad contestó `status=203/EXEC` al instalarse (ver #542).
//
// LA CONSECUENCIA NO ES «EL CLONE SALE ROTO Y NADIE SE ENTERA», Y CONVIENE DECIRLO BIEN: un
// `actions/checkout` materializa el modo del ÍNDICE, así que en CI el disco llega SIN el bit y la
// versión vieja también se ponía roja allá. El costo real era otro y es peor de diagnosticar: el
// rojo llegaba en CI diciendo «no es ejecutable» mientras el `ls -l` de quien lo rompió mostraba
// `-rwxrwxr-x`, o sea un rojo que no se reproduce en local y cuyo texto apunta al disco, que es
// justo donde no está el problema. Preguntándole al índice, el rojo aparece donde se rompió y el
// mensaje regala el comando que lo arregla.
//
// SE PREGUNTAN LAS DOS COSAS, y cada una tiene su propio sabotaje: que el guion esté EN EL DISCO
// (borrarlo rompe el árbol de quien trabaja) y que esté EN EL ÍNDICE con el bit (sin `git add` un
// archivo no existe para el repo, y sin el bit no se puede ejecutar allá). El modo del índice es
// independiente de la plataforma, así que desaparece el salteo de Windows: la guarda ahora mide
// lo mismo en las tres.
//
// Sabotaje: borrar cualquiera de los dos scripts, o sacarles el bit del índice.
// arnes: no_mecanizable="el sabotaje de la mitad nueva es un cambio de MODO en el índice de git (`git update-index --chmod=-x`), y el de la otra mitad es borrar un archivo: ninguno de los dos es una sustitución de texto, que es lo único que este arnés sabe aplicar"
func TestLasDosVerificacionesDeDespliegueExisten(t *testing.T) {
	modo := modosDelIndice(t, "deploy/")
	for _, s := range []string{"verificar-despliegue.sh", "verificar-cobertura.sh"} {
		if _, err := os.Stat(filepath.Join("..", "..", "deploy", s)); err != nil {
			t.Errorf("falta deploy/%s en el disco: %v", s, err)
		}
		m, ok := modo["deploy/"+s]
		if !ok {
			t.Errorf("deploy/%s no está en el índice de git.\n"+
				"  Sin `git add` no existe para el repo: el clone no lo recibe y la comprobación de "+
				"despliegue que este guion hace deja de correr allá, aunque acá el archivo esté.", s)
			continue
		}
		if m != "100755" {
			t.Errorf("deploy/%s está versionado como %s y tiene que ser 100755.\n"+
				"  El disco de esta máquina no dice nada: lo que viaja al clone es el modo del ÍNDICE, "+
				"y sin el bit una comprobación que hay que invocar con `bash` de por medio se corre "+
				"menos — y la que no se corre no existe.\n"+
				"  Arreglo: `git update-index --chmod=+x deploy/%s`.", s, m, s)
		}
	}
}

// modosDelIndice devuelve, para cada ruta versionada bajo `prefijo`, el modo con el que git la
// tiene guardada (`100644`, `100755`, …). Es LA fuente: el modo del disco puede estar parchado a
// mano mientras el del índice está roto, y es el del índice el que viaja al clone y al CI.
//
// Falla en vez de devolver poco: un mapa casi vacío se leería como «ningún archivo incumple».
func modosDelIndice(t *testing.T, prefijo string) map[string]string {
	t.Helper()
	salida, err := guiones.Herramienta(t, "git", "-C", filepath.Join("..", ".."), "ls-files", "-s", prefijo).Output()
	if err != nil {
		t.Fatalf("no se pudo leer el índice de git para %q: %v", prefijo, err)
	}
	modo := map[string]string{}
	for _, l := range strings.Split(string(salida), "\n") {
		if campos := strings.Fields(l); len(campos) >= 4 {
			modo[campos[3]] = campos[0]
		}
	}
	if len(modo) < 5 {
		t.Fatalf("el índice devolvió %d archivo(s) bajo %q y hay muchos más: el parseo dejó de "+
			"funcionar y esta guarda está midiendo el vacío", len(modo), prefijo)
	}
	return modo
}
