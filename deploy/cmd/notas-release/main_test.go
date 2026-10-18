package main

import (
	"bytes"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

// ---- ayudas ---------------------------------------------------------------------------------

// md convierte `´` en un backtick: un string crudo de Go no puede llevar backticks y los fixtures
// de Markdown los necesitan por todos lados.
func md(s string) string { return strings.ReplaceAll(s, "´", "`") }

// parsear lee un CHANGELOG de prueba (con `´` por backtick) y falla la prueba si no se puede.
func parsear(t *testing.T, texto string) []seccion {
	t.Helper()
	secs, err := parsearChangelog(md(texto))
	if err != nil {
		t.Fatalf("parsearChangelog: %v", err)
	}
	return secs
}

// titularDe pasa UNA viñeta (sin el «- » inicial y con las líneas de continuación ya sangradas)
// por el parser completo y devuelve su único titular.
func titularDe(t *testing.T, vineta string) string {
	t.Helper()
	secs := parsear(t, "## [1.0.0] - 2026-01-01\n\n### Added\n\n- "+vineta+"\n")
	if len(secs) != 1 || len(secs[0].Titulares["Added"]) != 1 {
		t.Fatalf("la viñeta %q no dio exactamente un titular: %+v", vineta, secs)
	}
	return secs[0].Titulares["Added"][0]
}

// correr ejecuta el comando en proceso y devuelve su código de salida y las dos salidas.
func correr(t *testing.T, args ...string) (codigo int, stdout, stderr string) {
	t.Helper()
	var so, se bytes.Buffer
	codigo = ejecutar(args, &so, &se)
	return codigo, so.String(), se.String()
}

func escribir(t *testing.T, dir, nombre, contenido string) string {
	t.Helper()
	ruta := filepath.Join(dir, nombre)
	if err := os.WriteFile(ruta, []byte(contenido), 0o644); err != nil {
		t.Fatal(err)
	}
	return ruta
}

func leer(t *testing.T, ruta string) string {
	t.Helper()
	b, err := os.ReadFile(ruta)
	if err != nil {
		t.Fatalf("no se pudo leer %s: %v", ruta, err)
	}
	return string(b)
}

var reModuloMusubi = regexp.MustCompile(`(?m)^module musubi\s*$`)

// raizDelRepo sube desde la carpeta del paquete hasta el go.mod con `module musubi`. Se busca por
// contenido y no con un `../../..` fijo: si el comando se mueve de carpeta, las pruebas contra los
// archivos reales no pueden quedar leyendo otro lado sin que nadie se entere.
func raizDelRepo(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if b, err := os.ReadFile(filepath.Join(dir, "go.mod")); err == nil && reModuloMusubi.Match(b) {
			return dir
		}
		padre := filepath.Dir(dir)
		if padre == dir {
			t.Fatalf("no encontré la raíz del repo (un go.mod con `module musubi`) subiendo desde %s", dir)
		}
		dir = padre
	}
}

// primeraDiferencia dice cuál es la primera línea que no coincide, con su número, para que el
// mensaje de un rojo apunte al lugar en vez de volcar dos archivos enteros.
func primeraDiferencia(actual, esperado string) string {
	la := strings.Split(strings.ReplaceAll(actual, "\r\n", "\n"), "\n")
	le := strings.Split(strings.ReplaceAll(esperado, "\r\n", "\n"), "\n")
	for i := 0; i < len(la) || i < len(le); i++ {
		switch {
		case i >= len(la):
			return fmt.Sprintf("línea %d: al archivo le falta %q", i+1, le[i])
		case i >= len(le):
			return fmt.Sprintf("línea %d: al archivo le sobra %q", i+1, la[i])
		case la[i] != le[i]:
			return fmt.Sprintf("línea %d:\n  tiene:   %q\n  debería: %q", i+1, la[i], le[i])
		}
	}
	return "las líneas son iguales: sólo difiere el tipo de salto de línea"
}

// reGuardaDeHerramientas es lo que mira internal/mcp/readme_toolcount_test.go en cada README
// (copiado: es un _test.go y no se puede importar), con las dos palabras y sin distinguir caja.
var reGuardaDeHerramientas = regexp.MustCompile(`(?i)(\d+)\s+(herramientas|tools)`)

// fueraDeCodigo devuelve s sin los code spans y avisa si quedó algún backtick sin pareja. Es la
// lectura de CommonMark hecha aparte de escanearCodigo a propósito: la prueba no le puede creer al
// código que está juzgando. Donde sacó un code span deja un carácter de control como separador:
// si no, dos `*` que quedaron a ambos lados de un code span se leerían pegados como un `**` que
// en Markdown nunca fue una negrita (`*` + code + `*` es un énfasis, no dos asteriscos juntos).
func fueraDeCodigo(s string) (fuera string, sinPareja bool) {
	type tira struct{ ini, fin int }
	var tiras []tira
	for i := 0; i < len(s); {
		if s[i] != '`' {
			i++
			continue
		}
		j := i
		for j < len(s) && s[j] == '`' {
			j++
		}
		tiras = append(tiras, tira{i, j})
		i = j
	}
	var b strings.Builder
	pos := 0
	for k := 0; k < len(tiras); {
		par := -1
		for m := k + 1; m < len(tiras); m++ {
			if tiras[m].fin-tiras[m].ini == tiras[k].fin-tiras[k].ini {
				par = m
				break
			}
		}
		if par < 0 {
			sinPareja = true
			b.WriteString(s[pos:tiras[k].fin])
			pos = tiras[k].fin
			k++
			continue
		}
		b.WriteString(s[pos:tiras[k].ini])
		b.WriteByte(0)
		pos = tiras[par].fin
		k = par + 1
	}
	b.WriteString(s[pos:])
	return b.String(), sinPareja
}

// problemaDeTitular devuelve "" si el titular cumple todo lo que el generador promete, o qué
// promesa rompe. Es la vara común de la prueba con entradas al azar y de la prueba contra el
// CHANGELOG real. Suma, a lo que se le exige a todo lo que se publica, el tope de 160 runas y que
// no termine en un signo suelto: cosas que un titular promete y una viñeta curada NO.
func problemaDeTitular(h string) string {
	switch {
	case utf8.RuneCountInString(h) > topeTitular+1:
		return fmt.Sprintf("mide %d runas y el tope es %d más el «…»", utf8.RuneCountInString(h), topeTitular)
	case h != "" && strings.IndexByte(" .:,;", h[len(h)-1]) >= 0 && !strings.HasSuffix(h, "&lt;"):
		return "termina en un signo suelto"
	}
	return problemaDeTexto(h)
}

// problemaDeTexto devuelve "" si una línea que el generador publica —un titular o una viñeta
// curada— cumple lo que se le promete a TODA línea publicada, o qué promesa rompe: una sola línea
// limpia, sin HTML suelto, con el código bien cerrado y sin ningún «N herramientas» a la vista de
// la guarda de los README.
func problemaDeTexto(h string) string {
	switch {
	case h == "":
		return "está vacío"
	case !utf8.ValidString(h):
		return "no es UTF-8 válido"
	case strings.ContainsAny(h, "\r\n\t"):
		return "tiene un salto de línea o una tabulación"
	case h != strings.TrimSpace(h) || strings.Contains(h, "  "):
		return "tiene espacios de más"
	case reGuardaDeHerramientas.MatchString(h):
		return "tiene «N herramientas» o «N tools» a la vista de la guarda de los README"
	}
	fuera, sinPareja := fueraDeCodigo(h)
	switch {
	case sinPareja:
		return "tiene un backtick sin pareja"
	case !strings.Contains(h, "``") && strings.Count(h, "`")%2 == 1:
		return "tiene una cantidad impar de backticks"
	case strings.Contains(fuera, "**"):
		return "tiene un `**` fuera de un code span"
	case strings.Contains(fuera, "<"):
		return "tiene un `<` suelto fuera de un code span"
	}
	return ""
}

// ---- fixtures -------------------------------------------------------------------------------

// changelogChico es un CHANGELOG mínimo con todo lo que el formato real tiene alrededor de las
// versiones: portada, `[Unreleased]`, un grupo `Notes` y el bloque de enlaces del final.
const changelogChico = `# Changelog

Texto de portada con ´## [8.8.8] - 2020-01-01´ en prosa, que no es una versión.

## [Unreleased]

### Added

- **Esto cuelga de Unreleased y no es de ninguna versión.** Detalle.

## [1.2.0] - 2026-09-14

### Added

- **Se agrega la primera novedad del producto.** Detalle uno.
- 🔴 **Se agrega la segunda novedad del producto.** Detalle dos.

### Fixed

- Se corrige el defecto que rompía el arranque. Y su detalle.

### Notes

- **Esta nota no es titular de nada.** Detalle.

## [1.1.0] - 2026-01-05

### Changed

- **Cambia el comportamiento del arranque inicial.** Detalle.

### Security

- **Se cierra el hueco de seguridad detectado.** Detalle.

[1.2.0]: https://example.com/1.2.0
[1.1.0]: https://example.com/1.1.0
`

const readmeBase = "# Proyecto\n\nIntro.\n\n## Novedades\n\n<!-- novedades:inicio -->\n<!-- novedades:fin -->\n\n## Más abajo\n\nTexto que no se toca.\n"

const (
	readmeEsChico = "# Proyecto\n\nIntro.\n\n## Novedades\n\n<!-- novedades:inicio -->\n\n" +
		"**[v1.2.0](https://github.com/o/r/releases/tag/v1.2.0)** · 14 sep 2026\n\n" +
		"- Se agrega la primera novedad del producto\n" +
		"- Se agrega la segunda novedad del producto\n" +
		"- Se corrige el defecto que rompía el arranque\n\n" +
		"**[v1.1.0](https://github.com/o/r/releases/tag/v1.1.0)** · 5 ene 2026\n\n" +
		"- Cambia el comportamiento del arranque inicial\n" +
		"- Se cierra el hueco de seguridad detectado\n\n" +
		"<!-- novedades:fin -->\n\n## Más abajo\n\nTexto que no se toca.\n"

	readmeEnChico = "# Proyecto\n\nIntro.\n\n## Novedades\n\n<!-- novedades:inicio -->\n\n" +
		"**[v1.2.0](https://github.com/o/r/releases/tag/v1.2.0)** · Sep 14, 2026\n\n" +
		"- Se agrega la primera novedad del producto\n" +
		"- Se agrega la segunda novedad del producto\n" +
		"- Se corrige el defecto que rompía el arranque\n\n" +
		"**[v1.1.0](https://github.com/o/r/releases/tag/v1.1.0)** · Jan 5, 2026\n\n" +
		"- Cambia el comportamiento del arranque inicial\n" +
		"- Se cierra el hueco de seguridad detectado\n\n" +
		"_The changelog is written in Spanish._\n\n" +
		"<!-- novedades:fin -->\n\n## Más abajo\n\nTexto que no se toca.\n"
)

// changelogConCercas esconde, dentro de cercas de código, todo lo que sería estructura si se
// leyera: versiones, grupos y viñetas en la columna 0. Una cerca de cuatro backticks contiene a
// una de tres sin cerrarse, y `~~~` también es cerca.
const changelogConCercas = `# Changelog

## [1.0.0] - 2026-01-02

### Added

- **Corre el comando así:**

  ´´´bash
  ## [9.9.9] - 2099-01-01
  ### Fixed
  - falso titular en la columna 0
  ´´´

- **Segundo titular real del añadido.** Con detalle.
  ´´´
  ## [9.9.8] - 2098-01-01
  ´´´

~~~
## [9.9.7] - 2097-01-01
### Removed
- otro falso
~~~

- **Tercer titular, después de las cercas.** Fin.

### Fixed

- Se corrige algo real. Y más.

´´´´
### Security
- falso dentro de una cerca de cuatro backticks
´´´
- sigue siendo falso
´´´´

- Se corrige otra cosa real.
`

// changelogDeCabezas junta los casos del párrafo de cabeza: continuación, línea en blanco,
// sub-viñetas de varios tipos y una lista que no es de este formato.
const changelogDeCabezas = `## [1.0.0] - 2026-01-02

### Added

- **Primero de los titulares con detalle.** Detalle en la misma línea.
  continuación sangrada que no es titular.

  Párrafo de detalle tras una línea en blanco: no es titular.
  - sub-viñeta que no es titular
    - sub-sub-viñeta que tampoco
- **Segundo titular con sub-viñeta inmediata**
  - sub-viñeta pegada a la cabeza
- Tercero sin negrita y sin punto final
  1. sub-lista numerada pegada a la cabeza
- Cuarto con continuación
  que sigue en la línea de abajo. Y más.
* Un asterisco en la columna cero no es una viñeta de este formato
`

// changelogDeEncabezados junta los encabezados que NO abren versión y los que sí, con sus formas.
const changelogDeEncabezados = `## [2.0.0] - 2026-02-02

### Added

#### Subtítulo de nivel cuatro

- **Sigue siendo del grupo Added tras un encabezado menor.** x

## Apéndice

### Added

- **Esto no es de ninguna versión: cuelga de un apéndice.** x

## [1.0.0]

### Fixed

- **Versión sin fecha en el encabezado.** x

## [1.0.0-rc.1](https://example.com/rc1) - 2025-12-31

### Added

- **Pre-release con enlace en el encabezado.** x

## [0.1] - 2025-01-01

### Added

- **No es una versión: sólo tiene dos componentes.** x
`

// vinetas arma n viñetas de primer nivel cuyo titular es «<rótulo> número i de la prueba»: la
// negrita pasa de las 15 runas, así que el titular es ella sola y no se le suma el resto.
func vinetas(rotulo string, n int) string {
	var b strings.Builder
	for i := 1; i <= n; i++ {
		fmt.Fprintf(&b, "- **%s número %d de la prueba.** Detalle que no entra en el titular.\n", rotulo, i)
	}
	return b.String()
}

// versionDePrueba arma una sección con la cantidad pedida de viñetas por grupo.
func versionDePrueba(version, fecha string, a, c, f, s, r, nota int) string {
	var b strings.Builder
	fmt.Fprintf(&b, "## [%s] - %s\n\n", version, fecha)
	for _, g := range []struct {
		grupo, rotulo string
		n             int
	}{
		{"Added", "Añadido", a}, {"Changed", "Cambiado", c}, {"Fixed", "Corregido", f},
		{"Security", "Seguridad", s}, {"Removed", "Eliminado", r}, {"Notes", "Nota", nota},
	} {
		if g.n > 0 {
			fmt.Fprintf(&b, "### %s\n\n%s\n", g.grupo, vinetas(g.rotulo, g.n))
		}
	}
	return b.String()
}

// changelogTopes tiene una versión con más titulares que los topes (19 contando sólo los cinco
// grupos que cuentan, más 5 de `Notes` que no cuentan), una que entra justo en el tope del README
// y una que lo pasa por uno.
var changelogTopes = "# Changelog\n\n## [Unreleased]\n\n### Added\n\n- **Colgado de Unreleased y de ninguna versión.** x\n\n" +
	versionDePrueba("3.0.0", "2026-03-05", 9, 2, 7, 1, 0, 5) +
	versionDePrueba("2.0.0", "2026-02-04", 4, 0, 0, 0, 0, 0) +
	versionDePrueba("1.0.0", "2026-01-02", 0, 0, 5, 0, 0, 0) +
	"[3.0.0]: https://example.com/3.0.0\n"

// ---- titulares ------------------------------------------------------------------------------

func TestTitular(t *testing.T) {
	casos := []struct{ nombre, entrada, esperado string }{
		{
			"negrita con frase",
			"**Se agrega la primera novedad del producto.** Detalle uno que no entra.",
			"Se agrega la primera novedad del producto",
		},
		{
			"negrita que cruza líneas",
			"**El titular empieza en una línea\n  y termina en la siguiente.** Detalle.",
			"El titular empieza en una línea y termina en la siguiente",
		},
		{
			"negrita corta con dos puntos adentro",
			"**Robustez:** el cargador ya no se cuelga con un archivo vacío. Y más detalle.",
			"Robustez: el cargador ya no se cuelga con un archivo vacío",
		},
		{
			"negrita corta con los dos puntos afuera",
			"**Contraste AA**: el color del texto secundario sube de nivel. Más.",
			"Contraste AA: el color del texto secundario sube de nivel",
		},
		{
			"negrita larga que termina en dos puntos es una etiqueta",
			"**Lo que cambia para quien actualiza:** hay que volver a correr el instalador. Más.",
			"Lo que cambia para quien actualiza: hay que volver a correr el instalador",
		},
		{
			"negrita larga con dos puntos en el medio no es etiqueta",
			"**Cuidado: esto cambia el comportamiento por defecto.** Resto.",
			"Cuidado: esto cambia el comportamiento por defecto",
		},
		{"negrita sola y corta", "**Hecho**", "Hecho"},
		{
			"emoji delante",
			"🔴 **La profundidad cambia el veredicto del examen.** Detalle.",
			"La profundidad cambia el veredicto del examen",
		},
		{
			"símbolo con selector de variación delante",
			"⚠️ **Cuidado con este cambio importante.** Detalle.",
			"Cuidado con este cambio importante",
		},
		{"emoji delante, sin negrita", "✅ Se ordena el arranque de la sesión. Detalle.", "Se ordena el arranque de la sesión"},
		{
			"sin negrita: la primera oración",
			"El regex del banco sólo aceptaba comillas dobles; al cambiar el motor dejó de aceptarlas. Más texto.",
			"El regex del banco sólo aceptaba comillas dobles; al cambiar el motor dejó de aceptarlas",
		},
		{
			"empieza con código y trae números con punto",
			"´designBriefBudget´ 6.000 → 6.600, y el motivo no es el tamaño. Resto.",
			"´designBriefBudget´ 6.000 → 6.600, y el motivo no es el tamaño",
		},
		{
			"«p. ej.» no termina la oración",
			"Los comandos ´ingest´ (p. ej. ´ingest url´) aparecen en la ayuda. Otra frase.",
			"Los comandos ´ingest´ (p. ej. ´ingest url´) aparecen en la ayuda",
		},
		{
			"un punto adentro de un code span no termina la oración",
			"El valor ´a. b´ se guarda en disco. Resto.",
			"El valor ´a. b´ se guarda en disco",
		},
		{
			"un enlace queda en su texto",
			"**Ver [la guía](https://x.example/z) para el detalle completo.** Más.",
			"Ver la guía para el detalle completo",
		},
		{
			"un enlace cuyo texto es código",
			"[´tools/call´](https://x.example/z) pasó a un mapa. Resto.",
			"´tools/call´ pasó a un mapa",
		},
		{
			"el `<` fuera de código se escapa y el de adentro no",
			"**Acepta <tag> y ´<code>´ sin romper el render.** Más.",
			"Acepta &lt;tag> y ´<code>´ sin romper el render",
		},
		{"un titular que termina en `<` no pierde el punto y coma", "Compara con el operador <", "Compara con el operador &lt;"},
		{
			"negrita sin cierre: la primera oración",
			"**Sin cierre de negrita pero con frase. Otra frase.",
			"Sin cierre de negrita pero con frase",
		},
		{
			"un `**` adentro de un code span no cierra la negrita",
			"**Usa ´a**b´ para algo importante.** Resto.",
			"Usa ´a**b´ para algo importante",
		},
		{
			"los corchetes de un code span no son un enlace",
			"**Soporta ´pydantic[extras]>=2.0´ en las dependencias.** x",
			"Soporta ´pydantic[extras]>=2.0´ en las dependencias",
		},
		{
			"las comillas de apertura se respetan",
			"«Un vector por trozo» queda medido y NO se construye. Más.",
			"«Un vector por trozo» queda medido y NO se construye",
		},
		{
			"espacios repetidos",
			"**Dos   espacios    seguidos en el titular.** x",
			"Dos espacios seguidos en el titular",
		},
		{
			"signos finales",
			"**Termina con varios signos sueltos ;,:.** x",
			"Termina con varios signos sueltos",
		},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			if got, want := titularDe(t, md(c.entrada)), md(c.esperado); got != want {
				t.Errorf("titular de %q\n  dio:      %q\n  esperaba: %q", c.entrada, got, want)
			}
		})
	}
}

func TestUnaVinetaSinNadaQueMostrarNoDaTitular(t *testing.T) {
	for _, entrada := range []string{"🔴", "**", "****", "   ", "→ ·"} {
		if got := titular(entrada); got != "" {
			t.Errorf("titular(%q) = %q; esperaba vacío", entrada, got)
		}
	}
	secs := parsear(t, "## [1.0.0] - 2026-01-01\n\n### Added\n\n- 🔴\n- **\n- **Esta sí tiene titular propio.** x\n")
	if got, want := secs[0].Titulares["Added"], []string{"Esta sí tiene titular propio"}; !reflect.DeepEqual(got, want) {
		t.Errorf("Added = %q; esperaba %q (una viñeta vacía no es un titular ni cuenta)", got, want)
	}
}

func TestTitularRecortaA160EnLimiteDePalabra(t *testing.T) {
	got := titularDe(t, "**"+strings.Repeat("palabra ", 40)+"final.** x")
	want := strings.TrimSpace(strings.Repeat("palabra ", 20)) + "…"
	if got != want {
		t.Errorf("dio %q (%d runas); esperaba %q (%d)", got, utf8.RuneCountInString(got), want, utf8.RuneCountInString(want))
	}
	// Justo en el tope no se corta: 160 runas exactas pasan enteras.
	justo := strings.Repeat("a", 159) + "b"
	if got := titularDe(t, "**"+justo+"** x"); got != justo {
		t.Errorf("160 runas exactas no se cortan: dio %q", got)
	}
}

func TestTitularNuncaDejaUnCodeSpanAMedias(t *testing.T) {
	// El corte natural cae adentro del code span: se corta ANTES del backtick de apertura.
	entrada := "**" + strings.Repeat("palabra ", 19) + "´un trozo de código largo que cruza el tope de los ciento sesenta´ final.** x"
	got := titularDe(t, md(entrada))
	want := strings.TrimSpace(strings.Repeat("palabra ", 19)) + "…"
	if got != want {
		t.Errorf("dio %q; esperaba %q", got, want)
	}
	if strings.Contains(got, "`") {
		t.Errorf("el titular quedó con un backtick: %q", got)
	}

	// Un code span que por sí solo pasa del tope no deja nada antes del backtick: el titular sale
	// sin él, y lo que estaba protegido (los `<`) se escapa sin pasarse del tope.
	largo := titularDe(t, md("´"+strings.Repeat("<", 200)+"´ y el resto de la frase."))
	if p := problemaDeTitular(largo); p != "" {
		t.Errorf("code span más largo que el tope: %q %s", largo, p)
	}
}

func TestUnTitularCortadoNoPierdeElPuntoYComaDelMenor(t *testing.T) {
	got := titularDe(t, strings.Repeat("a", 150)+" < "+strings.Repeat("b", 20))
	if want := strings.Repeat("a", 150) + " &lt;…"; got != want {
		t.Errorf("dio %q; esperaba %q (el `&lt;` tiene que quedar entero)", got, want)
	}
}

// ---- parser ---------------------------------------------------------------------------------

func TestSoloLasVersionesDelCHANGELOGSonVersiones(t *testing.T) {
	secs := parsear(t, changelogChico)
	var versiones []string
	for _, s := range secs {
		versiones = append(versiones, s.Version)
	}
	if want := []string{"1.2.0", "1.1.0"}; !reflect.DeepEqual(versiones, want) {
		t.Fatalf("versiones = %q; esperaba %q (ni `[Unreleased]` ni una versión citada en prosa cuentan)", versiones, want)
	}
	if got, want := secs[0].Fecha, "2026-09-14"; got != want {
		t.Errorf("fecha de 1.2.0 = %q; esperaba %q", got, want)
	}
	if want := strings.Count(changelogChico[:strings.Index(changelogChico, "## [1.2.0]")], "\n") + 1; secs[0].Linea != want {
		t.Errorf("línea del encabezado de 1.2.0 = %d; esperaba %d", secs[0].Linea, want)
	}
	want := map[string][]string{
		"Added": {"Se agrega la primera novedad del producto", "Se agrega la segunda novedad del producto"},
		"Fixed": {"Se corrige el defecto que rompía el arranque"},
	}
	if !reflect.DeepEqual(secs[0].Titulares, want) {
		t.Errorf("titulares de 1.2.0 = %q; esperaba %q (`Notes` no aporta nada)", secs[0].Titulares, want)
	}
}

func TestLasCercasDeCodigoNoSonEstructura(t *testing.T) {
	secs := parsear(t, changelogConCercas)
	if len(secs) != 1 || secs[0].Version != "1.0.0" {
		t.Fatalf("una versión de mentira dentro de una cerca se leyó como real: %+v", secs)
	}
	want := map[string][]string{
		"Added": {"Corre el comando así", "Segundo titular real del añadido", "Tercer titular, después de las cercas"},
		"Fixed": {"Se corrige algo real", "Se corrige otra cosa real"},
	}
	if !reflect.DeepEqual(secs[0].Titulares, want) {
		t.Errorf("titulares = %q\nesperaba   %q (lo de adentro de las cercas es literal)", secs[0].Titulares, want)
	}
}

func TestUnaCercaQueNoCierraEsUnError(t *testing.T) {
	_, err := parsearChangelog(md("## [1.0.0] - 2026-01-02\n\n### Added\n\n- **x.**\n  ´´´\n  ## [1.0.1] - 2026-01-03\n"))
	if err == nil || !strings.Contains(err.Error(), "línea 6") || !strings.Contains(err.Error(), "no se cierra") {
		t.Fatalf("error = %v; esperaba que nombrara la línea 6 y dijera que la cerca no se cierra", err)
	}
}

func TestUnaVersionRepetidaEsUnError(t *testing.T) {
	_, err := parsearChangelog("## [1.0.0] - 2026-01-02\n\n- **a.**\n\n## [0.9.0] - 2025-12-01\n\n## [1.0.0] - 2026-01-03\n")
	if err == nil || !strings.Contains(err.Error(), "1.0.0") || !strings.Contains(err.Error(), "dos veces") {
		t.Fatalf("error = %v; esperaba que dijera que la 1.0.0 aparece dos veces", err)
	}
}

func TestCRLFDaLoMismoQueLF(t *testing.T) {
	lf := parsear(t, changelogConCercas)
	crlf := parsear(t, strings.ReplaceAll(changelogConCercas, "\n", "\r\n"))
	if !reflect.DeepEqual(lf, crlf) {
		t.Errorf("el mismo CHANGELOG con CRLF dio otra cosa:\nLF:   %+v\nCRLF: %+v", lf, crlf)
	}
	if got := titularDe(t, "**Con salto de línea de Windows.**\r\n  Y su continuación. Fin."); strings.ContainsAny(got, "\r\n") {
		t.Errorf("el titular quedó con un salto de línea: %q", got)
	}
}

func TestElParrafoDeCabezaTerminaDondeEmpiezaElDetalle(t *testing.T) {
	secs := parsear(t, changelogDeCabezas)
	want := []string{
		"Primero de los titulares con detalle",
		"Segundo titular con sub-viñeta inmediata",
		"Tercero sin negrita y sin punto final",
		"Cuarto con continuación que sigue en la línea de abajo",
	}
	if got := secs[0].Titulares["Added"]; !reflect.DeepEqual(got, want) {
		t.Errorf("titulares = %q\nesperaba   %q (las sub-viñetas y los párrafos tras una línea en blanco no son titular)", got, want)
	}
}

func TestEncabezadosQueNoAbrenVersion(t *testing.T) {
	secs := parsear(t, changelogDeEncabezados)
	type visto struct{ version, fecha string }
	var got []visto
	for _, s := range secs {
		got = append(got, visto{s.Version, s.Fecha})
	}
	want := []visto{{"2.0.0", "2026-02-02"}, {"1.0.0", ""}, {"1.0.0-rc.1", "2025-12-31"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("versiones = %v; esperaba %v (un `## Apéndice` y un `## [0.1]` no son versiones)", got, want)
	}
	if n := len(secs[0].Titulares["Added"]); n != 1 {
		t.Errorf("tras un encabezado de nivel 4 el grupo Added tenía que seguir abierto: %d titulares", n)
	}
	if n := len(secs[1].Titulares["Added"]); n != 0 {
		t.Errorf("1.0.0 se quedó con %d titulares del apéndice", n)
	}
}

func TestNotesYLosGruposDesconocidosNoCuentan(t *testing.T) {
	secs := parsear(t, "## [1.0.0] - 2026-01-02\n\n### Added\n\n- **Un titular que sí cuenta.** x\n\n### Notes\n\n"+
		"- **Una nota.** x\n- **Otra nota.** x\n\n### Deprecated\n\n- **Un grupo que no está en la lista.** x\n\n### fixed\n\n- **La caja del grupo no importa.** x\n")
	s := secs[0]
	if got, want := enOrden(s), []string{"Un titular que sí cuenta", "La caja del grupo no importa"}; !reflect.DeepEqual(got, want) {
		t.Errorf("titulares = %q; esperaba %q", got, want)
	}
	for _, g := range []string{"Notes", "Deprecated"} {
		if _, hay := s.Titulares[g]; hay {
			t.Errorf("el grupo %s no tendría que aportar titulares", g)
		}
	}
	cuerpo := cuerpoRelease(s, textosEs, "o/r")
	for _, no := range []string{"nota", "Nota", "Deprecated", "Notes"} {
		if strings.Contains(cuerpo, no) {
			t.Errorf("el cuerpo de la release menciona %q:\n%s", no, cuerpo)
		}
	}
}

// ---- versiones, anclas y fechas -------------------------------------------------------------

func TestAnclaDeGitHub(t *testing.T) {
	casos := []struct{ encabezado, ancla string }{
		{"[0.141.0] - 2026-09-14", "01410---2026-09-14"},
		{"[1.0.0-rc.1] - 2026-01-02", "100-rc1---2026-01-02"},
		{"[1.0.0]", "100"},
		{"[1.0.0](https://example.com/x) - 2026-01-02", "100---2026-01-02"},
		{"[1.0.0] - Ñandú_X: ¡listo!", "100---ñandú_x-listo"},
	}
	for _, c := range casos {
		if got := anclaGitHub(c.encabezado); got != c.ancla {
			t.Errorf("anclaGitHub(%q) = %q; esperaba %q", c.encabezado, got, c.ancla)
		}
	}
	if got := parsear(t, "## [0.141.0] - 2026-09-14\n")[0].Ancla; got != "01410---2026-09-14" {
		t.Errorf("el ancla que guarda el parser es %q", got)
	}
}

func TestOrdenSemver(t *testing.T) {
	casos := []struct {
		a, b string
		want int
	}{
		{"0.9.0", "0.10.0", -1}, // como texto sería al revés
		{"0.10.0", "0.9.0", 1},
		{"1.0.0", "1.0.0", 0},
		{"2.0.0", "1.99.99", 1},
		{"1.0.0-rc.1", "1.0.0", -1},
		{"1.0.0", "1.0.0-rc.1", 1},
		{"1.0.0-rc.2", "1.0.0-rc.10", -1},
		{"1.0.0-alpha", "1.0.0-alpha.1", -1},
		{"1.0.0-1", "1.0.0-alpha", -1},
		{"1.0.0-beta", "1.0.0-alpha", 1},
		{"99999999999999999999.0.0", "99999999999999999998.0.0", 1}, // más grande que un int64
	}
	for _, c := range casos {
		if got := compararVersiones(c.a, c.b); (got > 0) != (c.want > 0) || (got < 0) != (c.want < 0) {
			t.Errorf("compararVersiones(%s, %s) = %d; esperaba el signo de %d", c.a, c.b, got, c.want)
		}
	}

	entrada := []seccion{{Version: "0.9.0"}, {Version: "0.10.0"}, {Version: "1.0.0-rc.1"}, {Version: "1.0.0"}, {Version: "0.2.0"}}
	copia := append([]seccion(nil), entrada...)
	var got []string
	for _, s := range ordenadas(entrada) {
		got = append(got, s.Version)
	}
	if want := []string{"1.0.0", "1.0.0-rc.1", "0.10.0", "0.9.0", "0.2.0"}; !reflect.DeepEqual(got, want) {
		t.Errorf("ordenadas = %q; esperaba %q", got, want)
	}
	if !reflect.DeepEqual(entrada, copia) {
		t.Errorf("ordenadas modificó su entrada: %+v", entrada)
	}
}

func TestFechasEnEspañolYEnIngles(t *testing.T) {
	es := []string{"ene", "feb", "mar", "abr", "may", "jun", "jul", "ago", "sep", "oct", "nov", "dic"}
	en := []string{"Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"}
	for m := 1; m <= 12; m++ {
		fecha := fmt.Sprintf("2026-%02d-%02d", m, m+2)
		if got, want := fechaCorta(fecha, textosEs), fmt.Sprintf("%d %s 2026", m+2, es[m-1]); got != want {
			t.Errorf("es %s = %q; esperaba %q", fecha, got, want)
		}
		if got, want := fechaCorta(fecha, textosEn), fmt.Sprintf("%s %d, 2026", en[m-1], m+2); got != want {
			t.Errorf("en %s = %q; esperaba %q", fecha, got, want)
		}
	}
	if got := fechaCorta("", textosEs); got != "" {
		t.Errorf("sin fecha dio %q", got)
	}
	if got := fechaCorta("2026-13-45", textosEs); got != "2026-13-45" {
		t.Errorf("una fecha ilegible tiene que salir tal cual y no perderse: %q", got)
	}
}

// ---- cuerpo de la release y bloque del README -----------------------------------------------

func TestCuerpoDeLaRelease(t *testing.T) {
	s := buscar(parsear(t, changelogChico), "1.2.0")
	enlace := "https://github.com/o/r/blob/v1.2.0/CHANGELOG.md#120---2026-09-14"

	wantEs := "### Añadido\n\n- Se agrega la primera novedad del producto\n- Se agrega la segunda novedad del producto\n\n" +
		"### Corregido\n\n- Se corrige el defecto que rompía el arranque\n\n" +
		"Detalle completo de esta versión en el [CHANGELOG](" + enlace + ").\n"
	if got := cuerpoRelease(*s, textosEs, "o/r"); got != wantEs {
		t.Errorf("cuerpo en español:\n%s\nesperaba:\n%s", got, wantEs)
	}

	wantEn := "### Added\n\n- Se agrega la primera novedad del producto\n- Se agrega la segunda novedad del producto\n\n" +
		"### Fixed\n\n- Se corrige el defecto que rompía el arranque\n\n" +
		"Full details for this release are in the [CHANGELOG](" + enlace + ") (written in Spanish).\n"
	if got := cuerpoRelease(*s, textosEn, "o/r"); got != wantEn {
		t.Errorf("cuerpo en inglés:\n%s\nesperaba:\n%s", got, wantEn)
	}
}

// esperadoDeGrupo arma el trozo de cuerpo de un grupo: `mostrados` titulares y, si hay más, la
// línea de resto con la plantilla `mas`.
func esperadoDeGrupo(titulo, rotulo string, mostrados, resto int, mas string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "### %s\n\n", titulo)
	for i := 1; i <= mostrados; i++ {
		fmt.Fprintf(&b, "- %s número %d de la prueba\n", rotulo, i)
	}
	if resto > 0 {
		fmt.Fprintf(&b, "- "+mas+"\n", resto)
	}
	return b.String() + "\n"
}

func TestLosTopesDelCuerpoDeLaRelease(t *testing.T) {
	secs := ordenadas(parsear(t, changelogTopes))
	s := buscar(secs, "3.0.0")
	enlace := "https://github.com/o/r/blob/v3.0.0/CHANGELOG.md#300---2026-03-05"

	wantEs := esperadoDeGrupo("Añadido", "Añadido", 6, 3, "… y %d más") +
		esperadoDeGrupo("Cambiado", "Cambiado", 2, 0, "") +
		esperadoDeGrupo("Corregido", "Corregido", 6, 1, "… y %d más") +
		esperadoDeGrupo("Seguridad", "Seguridad", 1, 0, "") +
		"Detalle completo de esta versión en el [CHANGELOG](" + enlace + ").\n"
	if got := cuerpoRelease(*s, textosEs, "o/r"); got != wantEs {
		t.Errorf("cuerpo en español:\n%s\nesperaba:\n%s", got, wantEs)
	}

	wantEn := esperadoDeGrupo("Added", "Añadido", 6, 3, "… and %d more") +
		esperadoDeGrupo("Changed", "Cambiado", 2, 0, "") +
		esperadoDeGrupo("Fixed", "Corregido", 6, 1, "… and %d more") +
		esperadoDeGrupo("Security", "Seguridad", 1, 0, "") +
		"Full details for this release are in the [CHANGELOG](" + enlace + ") (written in Spanish).\n"
	if got := cuerpoRelease(*s, textosEn, "o/r"); got != wantEn {
		t.Errorf("cuerpo en inglés:\n%s\nesperaba:\n%s", got, wantEn)
	}
	for _, no := range []string{"Nota número", "### Eliminado", "### Removed"} {
		if strings.Contains(wantEs+wantEn, no) {
			t.Fatalf("la propia prueba espera %q: revisá el fixture", no)
		}
	}
}

func TestLosTopesDelReadme(t *testing.T) {
	secs := ordenadas(parsear(t, changelogTopes))
	got := strings.Join(bloqueReadme(secs, 3, textosEs, "o/r"), "\n")
	want := "**[v3.0.0](https://github.com/o/r/releases/tag/v3.0.0)** · 5 mar 2026\n\n" +
		"- Añadido número 1 de la prueba\n- Añadido número 2 de la prueba\n- Añadido número 3 de la prueba\n- Añadido número 4 de la prueba\n\n" +
		// 9 + 2 + 7 + 1 = 19 titulares que cuentan, y `Notes` no suma: 19 - 4 = 15.
		"… y 15 más en el [CHANGELOG](https://github.com/o/r/blob/v3.0.0/CHANGELOG.md#300---2026-03-05)\n\n" +
		// Con exactamente 4 no hay línea de resto.
		"**[v2.0.0](https://github.com/o/r/releases/tag/v2.0.0)** · 4 feb 2026\n\n" +
		"- Añadido número 1 de la prueba\n- Añadido número 2 de la prueba\n- Añadido número 3 de la prueba\n- Añadido número 4 de la prueba\n\n" +
		"**[v1.0.0](https://github.com/o/r/releases/tag/v1.0.0)** · 2 ene 2026\n\n" +
		"- Corregido número 1 de la prueba\n- Corregido número 2 de la prueba\n- Corregido número 3 de la prueba\n- Corregido número 4 de la prueba\n\n" +
		"… y 1 más en el [CHANGELOG](https://github.com/o/r/blob/v1.0.0/CHANGELOG.md#100---2026-01-02)"
	if got != want {
		t.Errorf("bloque:\n%s\nesperaba:\n%s", got, want)
	}

	// -n recorta las versiones (las más nuevas primero) y pasarse de las que hay no rompe.
	if n := strings.Count(strings.Join(bloqueReadme(secs, 1, textosEs, "o/r"), "\n"), "**[v"); n != 1 {
		t.Errorf("con n=1 hay %d versiones", n)
	}
	if n := strings.Count(strings.Join(bloqueReadme(secs, 50, textosEs, "o/r"), "\n"), "**[v"); n != 3 {
		t.Errorf("con n=50 hay %d versiones y el CHANGELOG tiene 3", n)
	}
}

func TestElReadmeLlevaLasVersionesMasNuevasPorSemverYNoPorOrdenDelArchivo(t *testing.T) {
	// Pegada fuera de lugar: la 0.9.0 viene antes que la 0.10.0 en el archivo.
	texto := "## [0.9.0] - 2026-01-01\n\n### Added\n\n- **Se agrega la novedad de la nueve.** x\n\n" +
		"## [0.10.0] - 2026-02-01\n\n### Added\n\n- **Se agrega la novedad de la diez.** x\n"
	bloque := strings.Join(bloqueReadme(ordenadas(parsear(t, texto)), 1, textosEs, "o/r"), "\n")
	if !strings.Contains(bloque, "v0.10.0") || strings.Contains(bloque, "v0.9.0") {
		t.Errorf("con n=1 tenía que quedar la 0.10.0 y no la 0.9.0:\n%s", bloque)
	}
}

func TestUnaVersionSinFechaNoLlevaPuntoMedio(t *testing.T) {
	secs := parsear(t, "## [1.0.0]\n\n### Added\n\n- **Se agrega algo sin fecha en el encabezado.** x\n")
	if got := bloqueReadme(secs, 1, textosEs, "o/r")[0]; got != "**[v1.0.0](https://github.com/o/r/releases/tag/v1.0.0)**" {
		t.Errorf("cabecera = %q", got)
	}
}

func TestNingunaSalidaDiceNumeroHerramientas(t *testing.T) {
	// Las mismas dos expresiones que usa internal/mcp/readme_toolcount_test.go, una por README.
	guardas := []*regexp.Regexp{
		regexp.MustCompile(`\*\*(\d+)\s+herramientas\*\*|(\d+)\s+herramientas`),
		regexp.MustCompile(`\*\*(\d+)\s+tools\*\*|(\d+)\s+tools`),
	}
	secs := parsear(t, "## [1.0.0] - 2026-01-02\n\n### Added\n\n- **Pasamos de 43 a 58 herramientas en el catálogo.** Detalle.\n"+
		"- Se suman 7 tools nuevas al registro. Detalle.\n- Se documentan 3 herramientas.\n")
	salidas := []string{
		cuerpoRelease(secs[0], textosEs, "o/r"), cuerpoRelease(secs[0], textosEn, "o/r"),
		strings.Join(bloqueReadme(secs, 3, textosEs, "o/r"), "\n"), strings.Join(bloqueReadme(secs, 3, textosEn, "o/r"), "\n"),
	}
	for _, salida := range salidas {
		for _, re := range guardas {
			if m := re.FindString(salida); m != "" {
				t.Errorf("una salida del generador trae %q, que la guarda de los README leería como un conteo:\n%s", m, salida)
			}
		}
	}
	// Se ve igual: leído con el espacio de no separación como un espacio, el titular es el original.
	if got, want := strings.ReplaceAll(secs[0].Titulares["Added"][0], nbsp, " "), "Pasamos de 43 a 58 herramientas en el catálogo"; got != want {
		t.Errorf("el titular cambió de texto: %q", got)
	}
}

// ---- README: reemplazo entre marcadores -----------------------------------------------------

func TestReemplazoEntreMarcadores(t *testing.T) {
	bloque := []string{"línea uno", "", "línea dos"}
	got, err := reemplazarBloque(readmeBase, bloque)
	if err != nil {
		t.Fatal(err)
	}
	want := "# Proyecto\n\nIntro.\n\n## Novedades\n\n<!-- novedades:inicio -->\n\nlínea uno\n\nlínea dos\n\n<!-- novedades:fin -->\n\n## Más abajo\n\nTexto que no se toca.\n"
	if got != want {
		t.Errorf("README:\n%q\nesperaba:\n%q", got, want)
	}

	// Idempotente: lo que ya estaba escrito entre los marcadores se pisa y no se acumula.
	otra, err := reemplazarBloque(got, bloque)
	if err != nil || otra != got {
		t.Errorf("reemplazar dos veces cambió el resultado (err=%v):\n%q", err, otra)
	}

	// Y pisa lo que hubiera: contenido viejo, líneas en blanco de más.
	viejo := strings.Replace(readmeBase, marcaInicio+"\n", marcaInicio+"\n\n\nresto viejo\n- otro\n\n\n", 1)
	if otra, err := reemplazarBloque(viejo, bloque); err != nil || otra != want {
		t.Errorf("no pisó el contenido viejo (err=%v):\n%q", err, otra)
	}
}

func TestElReemplazoConservaElSaltoDeLineaDelArchivo(t *testing.T) {
	crlf := strings.ReplaceAll(readmeBase, "\n", "\r\n")
	got, err := reemplazarBloque(crlf, []string{"línea uno", "", "línea dos"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(strings.ReplaceAll(got, "\r\n", ""), "\n") {
		t.Errorf("quedó un salto LF suelto en un archivo CRLF:\n%q", got)
	}
	if !strings.HasPrefix(got, "# Proyecto\r\n\r\nIntro.") || !strings.HasSuffix(got, "Texto que no se toca.\r\n") {
		t.Errorf("lo de afuera de los marcadores cambió:\n%q", got)
	}
}

func TestLosMarcadoresSonLineasEnterasYSeValidan(t *testing.T) {
	casos := []struct {
		nombre, readme, error string
	}{
		{"sin inicio", "# x\n" + marcaFin + "\n", "falta el marcador " + marcaInicio},
		{"sin fin", "# x\n" + marcaInicio + "\n", "falta el marcador " + marcaFin},
		{"sin ninguno", "# x\n", "falta el marcador"},
		{"inicio repetido", marcaInicio + "\n" + marcaInicio + "\n" + marcaFin + "\n", "aparece 2 veces (líneas 1, 2)"},
		{"fin repetido", marcaInicio + "\n" + marcaFin + "\ntexto\n" + marcaFin + "\n", "aparece 2 veces (líneas 2, 4)"},
		{"fin antes que el inicio", marcaFin + "\n" + marcaInicio + "\n", "viene antes que"},
		{"sin publicar adentro", marcaInicio + "\n<!-- sin-publicar:inicio base=1.0.0 -->\n<!-- sin-publicar:fin -->\n" + marcaFin + "\n", "lo borraría"},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			_, err := reemplazarBloque(c.readme, []string{"x"})
			if err == nil || !strings.Contains(err.Error(), c.error) {
				t.Errorf("error = %v; esperaba que dijera %q", err, c.error)
			}
		})
	}

	// Nombrarlos en prosa (en un code span, en medio de una frase) no los duplica.
	prosa := "Los marcadores son ´" + marcaInicio + "´ y ´" + marcaFin + "´.\n\n" + readmeBase
	if _, err := reemplazarBloque(md(prosa), []string{"x"}); err != nil {
		t.Errorf("mencionar los marcadores en una frase se leyó como un marcador: %v", err)
	}
}

func TestElBloqueSinPublicarDeAfueraNoSeToca(t *testing.T) {
	sinPublicar := "<!-- sin-publicar:inicio base=1.2.0 -->\n### Sin publicar\n\n- Algo que está en main y todavía no salió.\n<!-- sin-publicar:fin -->\n"
	readme := sinPublicar + "\n" + readmeBase + "\n" + sinPublicar
	got, err := reemplazarBloque(readme, []string{"nuevo"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(got, sinPublicar) != 2 {
		t.Errorf("el generador tocó el bloque «sin publicar» de afuera:\n%s", got)
	}
}

// ---- README: guarda de `base=` --------------------------------------------------------------

func TestSinPublicarSeGuardaContraLaUltimaVersion(t *testing.T) {
	bloque := func(inicio string) string {
		return "# Proyecto\n\n" + inicio + "\n### Sin publicar\n\n- Algo.\n<!-- sin-publicar:fin -->\n"
	}
	casos := []struct {
		nombre, readme, ultima string
		partes                 []string // lo que tiene que decir el error; nil = tiene que pasar
	}{
		{"sin bloque no hay nada que guardar", "# Proyecto\n\nSin nada.\n", "1.2.0", nil},
		{"al día", bloque("<!-- sin-publicar:inicio base=1.2.0 -->"), "1.2.0", nil},
		{"al día con CRLF", strings.ReplaceAll(bloque("<!-- sin-publicar:inicio base=1.2.0 -->"), "\n", "\r\n"), "1.2.0", nil},
		{
			"una foto vieja", bloque("<!-- sin-publicar:inicio base=1.1.0 -->"), "1.2.0",
			// Lo que el mensaje tiene que decir: que la foto de `main` quedó vieja, cuál es la
			// versión nueva, y que se actualiza a mano o se borra el bloque y se cambia `base=`.
			[]string{"README.md:3", "foto de `main` que quedó vieja", "base=1.1.0", "1.2.0", "Actualizalo a mano", "cambiá `base=` a 1.2.0", "borrá el bloque"},
		},
		{"una foto de una versión más nueva que el CHANGELOG", bloque("<!-- sin-publicar:inicio base=1.3.0 -->"), "1.2.0", []string{"quedó vieja", "base=1.3.0"}},
		{"base con v", bloque("<!-- sin-publicar:inicio base=v1.2.0 -->"), "1.2.0", []string{"no tiene la forma X.Y.Z"}},
		{"base vacía", bloque("<!-- sin-publicar:inicio base= -->"), "1.2.0", []string{"no tiene la forma X.Y.Z"}},
		{"base incompleta", bloque("<!-- sin-publicar:inicio base=1.2 -->"), "1.2.0", []string{"no tiene la forma X.Y.Z"}},
		{"marcador de inicio con otra forma", bloque("<!-- sin-publicar:inicio base=1.2.0  -->"), "1.2.0", []string{"exactamente"}},
		{"sin cierre", "<!-- sin-publicar:inicio base=1.2.0 -->\n- x\n", "1.2.0", []string{"abrir una vez", "cerrar una vez"}},
		{"inicio repetido", bloque("<!-- sin-publicar:inicio base=1.2.0 -->\n<!-- sin-publicar:inicio base=1.2.0 -->"), "1.2.0", []string{"abrir una vez"}},
		{"cierre antes que el inicio", "<!-- sin-publicar:fin -->\n<!-- sin-publicar:inicio base=1.2.0 -->\n", "1.2.0", []string{"en ese orden"}},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			err := comprobarSinPublicar(c.readme, "README.md", c.ultima)
			if c.partes == nil {
				if err != nil {
					t.Errorf("tenía que pasar y falló: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("tenía que fallar y pasó")
			}
			for _, parte := range c.partes {
				if !strings.Contains(err.Error(), parte) {
					t.Errorf("el error no dice %q:\n%v", parte, err)
				}
			}
		})
	}
}

// ---- el comando de punta a punta ------------------------------------------------------------

func TestUsoIncorrectoSaleConDosYSinNadaEnStdout(t *testing.T) {
	casos := []struct {
		nombre  string
		args    []string
		mensaje string
	}{
		{"sin nada", nil, "exactamente una cosa"},
		{"las dos a la vez", []string{"-version", "1.0.0", "-readme"}, "exactamente una cosa"},
		{"versión mal formada", []string{"-version", "1.0"}, "X.Y.Z"},
		{"versión con texto de más", []string{"-version", "1.0.0; rm -rf"}, "X.Y.Z"},
		{"idioma que no existe", []string{"-version", "1.0.0", "-lang", "fr"}, "es o en"},
		{"repo mal formado", []string{"-version", "1.0.0", "-repo", "sinbarra"}, "dueño/nombre"},
		{"n en cero", []string{"-readme", "-n", "0"}, "1 o más"},
		{"argumentos de más", []string{"-version", "1.0.0", "sobra"}, "argumentos de más"},
		{"flag que no existe", []string{"-nada"}, "not defined"},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			codigo, out, errOut := correr(t, c.args...)
			if codigo != 2 {
				t.Errorf("código = %d; esperaba 2", codigo)
			}
			if out != "" {
				t.Errorf("stdout tenía que estar vacío: %q", out)
			}
			if !strings.Contains(errOut, c.mensaje) || !strings.Contains(errOut, "go run ./deploy/cmd/notas-release") {
				t.Errorf("stderr no dice %q ni muestra el uso:\n%s", c.mensaje, errOut)
			}
		})
	}
	if codigo, _, errOut := correr(t, "-h"); codigo != 0 || !strings.Contains(errOut, "-version") {
		t.Errorf("-h: código %d, stderr %q", codigo, errOut)
	}
}

func TestVersionConoSinV(t *testing.T) {
	cl := escribir(t, t.TempDir(), "CHANGELOG.md", changelogChico)
	_, sin, _ := correr(t, "-changelog", cl, "-repo", "o/r", "-version", "1.2.0")
	codigo, con, errOut := correr(t, "-changelog", cl, "-repo", "o/r", "-version", "v1.2.0")
	if codigo != 0 || errOut != "" || con != sin || con == "" {
		t.Errorf("con v: código %d, stderr %q; sin v y con v tienen que dar lo mismo:\n%s\n---\n%s", codigo, errOut, sin, con)
	}
}

func TestVersionDaElCuerpoDeLaRelease(t *testing.T) {
	cl := escribir(t, t.TempDir(), "CHANGELOG.md", changelogChico)
	enlace := "https://github.com/o/r/blob/v1.2.0/CHANGELOG.md#120---2026-09-14"
	codigo, out, errOut := correr(t, "-changelog", cl, "-repo", "o/r", "-version", "1.2.0")
	want := "### Añadido\n\n- Se agrega la primera novedad del producto\n- Se agrega la segunda novedad del producto\n\n" +
		"### Corregido\n\n- Se corrige el defecto que rompía el arranque\n\n" +
		"Detalle completo de esta versión en el [CHANGELOG](" + enlace + ").\n"
	if codigo != 0 || errOut != "" || out != want {
		t.Errorf("código %d, stderr %q, stdout:\n%s\nesperaba:\n%s", codigo, errOut, out, want)
	}
	codigo, out, errOut = correr(t, "-changelog", cl, "-repo", "o/r", "-version", "1.2.0", "-lang", "en")
	if codigo != 0 || errOut != "" || !strings.HasPrefix(out, "### Added\n") || !strings.HasSuffix(out, "(written in Spanish).\n") {
		t.Errorf("en inglés: código %d, stderr %q, stdout:\n%s", codigo, errOut, out)
	}
}

func TestVersionSinSeccionDaUnaLineaDeReservaYConEstrictoFalla(t *testing.T) {
	cl := escribir(t, t.TempDir(), "CHANGELOG.md", changelogChico)

	codigo, out, errOut := correr(t, "-changelog", cl, "-repo", "o/r", "-version", "1.1.5")
	want := "Esta versión no tiene sección propia en el [CHANGELOG](https://github.com/o/r/blob/v1.1.5/CHANGELOG.md).\n"
	if codigo != 0 || out != want || errOut != "" {
		t.Errorf("reserva: código %d, stdout %q, stderr %q; esperaba 0, %q y nada", codigo, out, errOut, want)
	}
	codigo, out, _ = correr(t, "-changelog", cl, "-repo", "o/r", "-version", "1.1.5", "-lang", "en")
	if codigo != 0 || !strings.HasPrefix(out, "This release has no section of its own in the [CHANGELOG](") || strings.Count(out, "\n") != 1 {
		t.Errorf("reserva en inglés: código %d, stdout %q", codigo, out)
	}

	codigo, out, errOut = correr(t, "-changelog", cl, "-repo", "o/r", "-version", "1.1.5", "-estricto")
	if codigo != 1 || out != "" || !strings.Contains(errOut, "1.1.5") {
		t.Errorf("-estricto: código %d, stdout %q, stderr %q; esperaba 1, nada y un mensaje con la versión", codigo, out, errOut)
	}
	// Con una versión que sí existe, -estricto no cambia nada.
	codigo, out, _ = correr(t, "-changelog", cl, "-repo", "o/r", "-version", "1.2.0", "-estricto")
	if codigo != 0 || !strings.HasPrefix(out, "### Añadido") {
		t.Errorf("-estricto con una versión que existe: código %d, stdout %q", codigo, out)
	}
}

func TestUnCHANGELOGIlegibleFallaDiciendoQue(t *testing.T) {
	codigo, out, errOut := correr(t, "-changelog", filepath.Join(t.TempDir(), "no-existe.md"), "-version", "1.0.0")
	if codigo != 1 || out != "" || !strings.Contains(errOut, "no se pudo leer el CHANGELOG") {
		t.Errorf("código %d, stdout %q, stderr %q", codigo, out, errOut)
	}
	cl := escribir(t, t.TempDir(), "CHANGELOG.md", md("## [1.0.0] - 2026-01-02\n\n- **x.**\n´´´\nsin cerrar\n"))
	if codigo, _, errOut := correr(t, "-changelog", cl, "-version", "1.0.0"); codigo != 1 || !strings.Contains(errOut, "no se cierra") {
		t.Errorf("con una cerca sin cerrar: código %d, stderr %q", codigo, errOut)
	}
}

func TestReadmeEscribeLosDosArchivosYEsIdempotente(t *testing.T) {
	dir := t.TempDir()
	cl := escribir(t, dir, "CHANGELOG.md", changelogChico)
	es := escribir(t, dir, "README.md", readmeBase)
	en := escribir(t, dir, "README.en.md", readmeBase)
	args := []string{"-readme", "-changelog", cl, "-root", dir, "-repo", "o/r"}

	codigo, out, errOut := correr(t, args...)
	if codigo != 0 || errOut != "" {
		t.Fatalf("código %d, stderr %q", codigo, errOut)
	}
	if lineas := strings.Split(strings.TrimSpace(out), "\n"); len(lineas) != 2 ||
		!strings.HasSuffix(lineas[0], "README.md: actualizado") || !strings.HasSuffix(lineas[1], "README.en.md: actualizado") {
		t.Errorf("salida inesperada:\n%s", out)
	}
	if got := leer(t, es); got != readmeEsChico {
		t.Errorf("README.md (%s):\n%s", primeraDiferencia(got, readmeEsChico), got)
	}
	if got := leer(t, en); got != readmeEnChico {
		t.Errorf("README.en.md (%s):\n%s", primeraDiferencia(got, readmeEnChico), got)
	}

	// Segunda corrida: no cambia nada y lo dice.
	codigo, out, _ = correr(t, args...)
	if codigo != 0 || strings.Count(out, "sin cambios") != 2 || strings.Contains(out, "actualizado") {
		t.Errorf("la segunda corrida tenía que dar «sin cambios» dos veces: código %d\n%s", codigo, out)
	}
	if leer(t, es) != readmeEsChico || leer(t, en) != readmeEnChico {
		t.Error("la segunda corrida cambió un README")
	}
}

func TestReadmeConNMuestraMenosVersiones(t *testing.T) {
	dir := t.TempDir()
	cl := escribir(t, dir, "CHANGELOG.md", changelogChico)
	es := escribir(t, dir, "README.md", readmeBase)
	escribir(t, dir, "README.en.md", readmeBase)
	if codigo, _, errOut := correr(t, "-readme", "-n", "1", "-changelog", cl, "-root", dir); codigo != 0 {
		t.Fatalf("código %d: %s", codigo, errOut)
	}
	got := leer(t, es)
	if strings.Count(got, "**[v") != 1 || !strings.Contains(got, "v1.2.0") {
		t.Errorf("con -n 1 tenía que quedar sólo la versión más nueva:\n%s", got)
	}
}

// El README tiene que mostrar las versiones más nuevas por semver, no las primeras del archivo: el
// CHANGELOG real viene ordenado, así que sólo un fixture desordenado y el comando entero lo prueban.
func TestReadmeOrdenaPorSemverAunqueElArchivoNo(t *testing.T) {
	dir := t.TempDir()
	cl := escribir(t, dir, "CHANGELOG.md", "## [0.9.0] - 2026-01-01\n\n### Added\n\n- **Se agrega la novedad de la nueve.** x\n\n"+
		"## [0.10.0] - 2026-02-01\n\n### Added\n\n- **Se agrega la novedad de la diez.** x\n")
	es := escribir(t, dir, "README.md", readmeBase)
	escribir(t, dir, "README.en.md", readmeBase)

	if codigo, _, errOut := correr(t, "-readme", "-n", "1", "-changelog", cl, "-root", dir); codigo != 0 {
		t.Fatalf("código %d: %s", codigo, errOut)
	}
	if got := leer(t, es); !strings.Contains(got, "v0.10.0") || strings.Contains(got, "v0.9.0") {
		t.Errorf("con -n 1 tenía que quedar la 0.10.0 y no la 0.9.0:\n%s", got)
	}
	if codigo, _, errOut := correr(t, "-readme", "-n", "2", "-changelog", cl, "-root", dir); codigo != 0 {
		t.Fatalf("código %d: %s", codigo, errOut)
	}
	if got := leer(t, es); !strings.Contains(got, "v0.9.0") || strings.Index(got, "v0.10.0") > strings.Index(got, "v0.9.0") {
		t.Errorf("con -n 2 las dos versiones tienen que estar y la 0.10.0 va primero:\n%s", got)
	}
}

func TestReadmeNoEscribeNingunoSiUnoTieneLosMarcadoresRotos(t *testing.T) {
	dir := t.TempDir()
	cl := escribir(t, dir, "CHANGELOG.md", changelogChico)
	es := escribir(t, dir, "README.md", readmeBase)
	escribir(t, dir, "README.en.md", "# Sin marcadores\n")

	codigo, _, errOut := correr(t, "-readme", "-changelog", cl, "-root", dir)
	if codigo != 1 || !strings.Contains(errOut, "README.en.md") || !strings.Contains(errOut, "falta el marcador") {
		t.Errorf("código %d, stderr %q; esperaba 1 y un error que nombrara README.en.md", codigo, errOut)
	}
	if got := leer(t, es); got != readmeBase {
		t.Errorf("README.md se escribió aunque el otro estaba roto:\n%s", got)
	}
}

func TestReadmeSinVersionesEnElCHANGELOGFalla(t *testing.T) {
	dir := t.TempDir()
	cl := escribir(t, dir, "CHANGELOG.md", "# Changelog\n\n## [Unreleased]\n\n- **x.**\n")
	es := escribir(t, dir, "README.md", readmeBase)
	escribir(t, dir, "README.en.md", readmeBase)
	if codigo, _, errOut := correr(t, "-readme", "-changelog", cl, "-root", dir); codigo != 1 || !strings.Contains(errOut, "ninguna versión") {
		t.Errorf("código %d, stderr %q", codigo, errOut)
	}
	if leer(t, es) != readmeBase {
		t.Error("se escribió un README sin tener de dónde sacar versiones")
	}
}

// ---- grupos curados: Destacado y Highlights -------------------------------------------------

// changelogCurado tiene tres versiones: la 3.0.0 con Highlights Y Destacado —en ese orden, a
// propósito: la posición dentro de la versión no importa— y titulares de los grupos de siempre a
// los costados, la 2.0.0 con sólo Destacado y la 1.0.0 sin ninguno. Más un Destacado colgado de
// `[Unreleased]`, que no es de ninguna versión.
const changelogCurado = `# Changelog

## [Unreleased]

### Destacado

- Esto cuelga de Unreleased y no es de ninguna versión.

## [3.0.0] - 2026-03-05

### Added

- **Se agrega la primera novedad de la tres.** Detalle uno.
- **Se agrega la segunda novedad de la tres.** Detalle dos.

### Highlights

- First curated idea of three, with its final period.
- Second curated idea of three.

### Destacado

- Primera idea curada de la tres, con su punto final.
- Segunda idea curada de la tres.

### Fixed

- **Se corrige el defecto de la tres.** Detalle.

## [2.0.0] - 2026-02-04

### Added

- **Se agrega la novedad de la dos.** Detalle.

### Destacado

- Única idea curada de la dos, sin versión en inglés.

## [1.0.0] - 2026-01-02

### Added

- **Se agrega la novedad de la uno.** Detalle.
`

const (
	// Lo que tiene que salir de changelogCurado en el bloque de Novedades de cada README (las tres
	// versiones, repo o/r).
	bloqueEsCurado = "**[v3.0.0](https://github.com/o/r/releases/tag/v3.0.0)** · 5 mar 2026\n\n" +
		"- Primera idea curada de la tres, con su punto final.\n" +
		"- Segunda idea curada de la tres.\n\n" +
		"Y todo lo demás, en el [CHANGELOG](https://github.com/o/r/blob/v3.0.0/CHANGELOG.md#300---2026-03-05)\n\n" +
		"**[v2.0.0](https://github.com/o/r/releases/tag/v2.0.0)** · 4 feb 2026\n\n" +
		"- Única idea curada de la dos, sin versión en inglés.\n\n" +
		"Y todo lo demás, en el [CHANGELOG](https://github.com/o/r/blob/v2.0.0/CHANGELOG.md#200---2026-02-04)\n\n" +
		"**[v1.0.0](https://github.com/o/r/releases/tag/v1.0.0)** · 2 ene 2026\n\n" +
		"- Se agrega la novedad de la uno"

	bloqueEnCurado = "**[v3.0.0](https://github.com/o/r/releases/tag/v3.0.0)** · Mar 5, 2026\n\n" +
		"- First curated idea of three, with its final period.\n" +
		"- Second curated idea of three.\n\n" +
		"And everything else, in the [CHANGELOG](https://github.com/o/r/blob/v3.0.0/CHANGELOG.md#300---2026-03-05)\n\n" +
		"**[v2.0.0](https://github.com/o/r/releases/tag/v2.0.0)** · Feb 4, 2026\n\n" +
		"- Única idea curada de la dos, sin versión en inglés.\n\n" +
		"And everything else, in the [CHANGELOG](https://github.com/o/r/blob/v2.0.0/CHANGELOG.md#200---2026-02-04)\n\n" +
		"**[v1.0.0](https://github.com/o/r/releases/tag/v1.0.0)** · Jan 2, 2026\n\n" +
		"- Se agrega la novedad de la uno\n\n" +
		"_The changelog is written in Spanish._"

	// Y el cuerpo de la release 3.0.0 en cada idioma.
	enlaceCurado3 = "https://github.com/o/r/blob/v3.0.0/CHANGELOG.md#300---2026-03-05"

	cuerpoEsCurado3 = "### Destacado\n\n- Primera idea curada de la tres, con su punto final.\n- Segunda idea curada de la tres.\n\n" +
		"### Añadido\n\n- Se agrega la primera novedad de la tres\n- Se agrega la segunda novedad de la tres\n\n" +
		"### Corregido\n\n- Se corrige el defecto de la tres\n\n" +
		"Detalle completo de esta versión en el [CHANGELOG](" + enlaceCurado3 + ").\n"

	cuerpoEnCurado3 = "### Highlights\n\n- First curated idea of three, with its final period.\n- Second curated idea of three.\n\n" +
		"### Added\n\n- Se agrega la primera novedad de la tres\n- Se agrega la segunda novedad de la tres\n\n" +
		"### Fixed\n\n- Se corrige el defecto de la tres\n\n" +
		"Full details for this release are in the [CHANGELOG](" + enlaceCurado3 + ") (written in Spanish).\n"
)

// readmeCon devuelve readmeBase con el bloque de Novedades puesto entre los marcadores.
func readmeCon(bloque string) string {
	return strings.Replace(readmeBase, marcaInicio+"\n"+marcaFin+"\n", marcaInicio+"\n\n"+bloque+"\n\n"+marcaFin+"\n", 1)
}

// curadasN arma n viñetas curadas de una sola línea, «<prefijo> número i de la prueba.».
func curadasN(prefijo string, n int) string {
	var b strings.Builder
	for i := 1; i <= n; i++ {
		fmt.Fprintf(&b, "- %s número %d de la prueba.\n", prefijo, i)
	}
	return b.String()
}

func TestGrupoCanonicoReconoceDestacadoYHighlights(t *testing.T) {
	casos := []struct{ titulo, want string }{
		{"Destacado", "Destacado"}, {"destacado", "Destacado"}, {"DESTACADO", "Destacado"},
		{"Highlights", "Highlights"}, {"highlights", "Highlights"}, {"HiGhLiGhTs", "Highlights"},
		{"Added", "Added"}, {"fixed", "Fixed"},
		// Lo que NO se reconoce: el plural, el singular inglés, un agregado y lo de siempre.
		{"Destacados", ""}, {"Highlight", ""}, {"Destacado (resumen)", ""}, {"Notes", ""}, {"", ""},
	}
	for _, c := range casos {
		if got := grupoCanonico(c.titulo); got != c.want {
			t.Errorf("grupoCanonico(%q) = %q; esperaba %q", c.titulo, got, c.want)
		}
	}
}

func TestElEncabezadoCuradoNoDistingueCajaNiPosicion(t *testing.T) {
	secs := parsear(t, "## [1.0.0] - 2026-01-02\n\n"+
		"### HIGHLIGHTS\n\n- First idea.\n\n"+
		"### Added\n\n- **Se agrega algo real de la prueba.** x\n\n"+
		"### destacado\n\n- Primera idea.\n\n"+
		"### Fixed\n\n- **Se corrige algo real de la prueba.** x\n")
	s := secs[0]
	if want := []string{"Primera idea."}; !reflect.DeepEqual(s.Destacado, want) {
		t.Errorf("Destacado = %q; esperaba %q", s.Destacado, want)
	}
	if want := []string{"First idea."}; !reflect.DeepEqual(s.Highlights, want) {
		t.Errorf("Highlights = %q; esperaba %q", s.Highlights, want)
	}
	// Lo curado no se mezcla con los titulares derivados de los grupos de siempre.
	want := map[string][]string{"Added": {"Se agrega algo real de la prueba"}, "Fixed": {"Se corrige algo real de la prueba"}}
	if !reflect.DeepEqual(s.Titulares, want) {
		t.Errorf("titulares = %q; esperaba %q (Destacado y Highlights no son un grupo de titulares)", s.Titulares, want)
	}
}

func TestUnaVinetaCuradaSeGuardaCompletaYNoComoTitular(t *testing.T) {
	larga := strings.TrimSpace(strings.Repeat("palabra ", 40)) + " final."
	secs := parsear(t, "## [1.0.0] - 2026-01-02\n\n### Destacado\n\n"+
		"- Primera oración. Segunda oración de la misma viñeta.\n"+
		"- "+larga+"\n"+
		"- **Etiqueta corta.** El resto, con [un enlace](https://example.com/x) y ´código <b>´.\n\n"+
		"### Highlights\n\n- Same idea, in English. With two sentences.\n")
	s := secs[0]
	want := []string{
		"Primera oración. Segunda oración de la misma viñeta.", // un titular se quedaría con «Primera oración»
		larga, // y esta, de 326 runas, saldría cortada a 160 con «…»
		md("Etiqueta corta. El resto, con un enlace y ´código <b>´."),
	}
	if !reflect.DeepEqual(s.Destacado, want) {
		t.Errorf("Destacado:\n%q\nesperaba:\n%q", s.Destacado, want)
	}
	if want := []string{"Same idea, in English. With two sentences."}; !reflect.DeepEqual(s.Highlights, want) {
		t.Errorf("Highlights = %q; esperaba %q", s.Highlights, want)
	}
	if len(s.Titulares) != 0 {
		t.Errorf("lo curado dejó titulares derivados: %q", s.Titulares)
	}
	if utf8.RuneCountInString(larga) <= topeTitular+1 {
		t.Fatal("la propia prueba no sirve: la viñeta larga entra en el tope de un titular")
	}
}

func TestUnaVinetaCuradaPuedeOcuparVariasLineas(t *testing.T) {
	secs := parsear(t, "## [1.0.0] - 2026-01-02\n\n### Destacado\n\n"+
		"- El grafo de código se reindexa solo, al arrancar\n"+
		"  y después cada hora (antes cada 6),\n"+
		"  y sólo se manda al central cuando hay algo nuevo.\n"+
		"  - sub-viñeta que no cuenta como viñeta del grupo\n"+
		"    - ni esta, más adentro\n"+
		"- Segunda viñeta de una sola línea.\n\n"+
		"  Párrafo suelto tras una línea en blanco: es detalle, no parte de la viñeta.\n"+
		"- Tercera, con una lista numerada pegada:\n"+
		"  1. que no cuenta\n"+
		"- Cuarta con continuación sin sangría\n"+
		"que igual se une, como en CommonMark.\n"+
		"* Un asterisco en la columna cero no es una viñeta de este formato\n")
	want := []string{
		"El grafo de código se reindexa solo, al arrancar y después cada hora (antes cada 6), y sólo se manda al central cuando hay algo nuevo.",
		"Segunda viñeta de una sola línea.",
		"Tercera, con una lista numerada pegada:",
		"Cuarta con continuación sin sangría que igual se une, como en CommonMark.",
	}
	if got := secs[0].Destacado; !reflect.DeepEqual(got, want) {
		t.Errorf("Destacado:\n%q\nesperaba:\n%q (las líneas de continuación se unen con un espacio; las sub-viñetas no cuentan)", got, want)
	}
}

func TestUnGrupoCuradoVacioEsComoSiNoExistiera(t *testing.T) {
	casos := []struct{ nombre, grupo string }{
		{"sin viñetas", "### Destacado\n\n"},
		{"sólo prosa", "### Destacado\n\nUn párrafo que no es una viñeta.\n\n"},
		{"sólo una sub-viñeta", "### Destacado\n\n  - sangrada: no es de primer nivel\n\n"},
		{"viñetas que no dejan nada", "### Destacado\n\n- **\n- ´´\n- \n\n"},
		{"un asterisco en la columna cero", "### Destacado\n\n* no es una viñeta de este formato\n\n"},
		{"Highlights vacío", "### Highlights\n\n"},
		{"Highlights con viñetas que no dejan nada", "### Highlights\n\n- **\n- ´´\n- \n\n"},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			secs := parsear(t, "## [1.0.0] - 2026-01-02\n\n"+c.grupo+"### Added\n\n- **Se agrega algo real de la prueba.** x\n")
			s := secs[0]
			if len(s.Destacado) != 0 || len(s.Highlights) != 0 {
				t.Fatalf("un grupo curado sin nada que mostrar dejó viñetas: %q / %q", s.Destacado, s.Highlights)
			}
			// Es como si no existiera: ni encabezado propio en la release ni cierre curado en el README.
			for _, t0 := range []textos{textosEs, textosEn} {
				cuerpo := cuerpoRelease(s, t0, "o/r")
				if strings.Contains(cuerpo, "### Destacado") || strings.Contains(cuerpo, "### Highlights") {
					t.Errorf("el cuerpo trae un bloque curado vacío:\n%s", cuerpo)
				}
				bloque := strings.Join(bloqueReadme(secs, 1, t0, "o/r"), "\n")
				if strings.Contains(bloque, "todo lo demás") || strings.Contains(bloque, "everything else") ||
					!strings.Contains(bloque, "- Se agrega algo real de la prueba") || !strings.Contains(bloque, "**[v1.0.0]") {
					t.Errorf("el README no cayó en el fallback:\n%s", bloque)
				}
			}
		})
	}
}

// Una viñeta suelta, sin `###` propio, bajo el encabezado de una versión no es de ningún grupo: no
// hereda el grupo en el que terminó la versión anterior del archivo. Para lo curado el riesgo es
// concreto: una línea de prosa pegada al `## [x]` saldría como viñeta destacada de esa versión.
func TestUnaVinetaSueltaNoHeredaElGrupoDeLaVersionAnterior(t *testing.T) {
	secs := parsear(t, "## [3.0.0] - 2026-03-01\n\n"+
		"- Suelta de la 3.0.0, bajo el encabezado y sin grupo.\n\n"+
		"### Destacado\n\n- Idea de la 3.0.0.\n\n"+
		"## [2.0.0] - 2026-02-01\n\n"+
		"- Suelta de la 2.0.0: no hereda el Destacado de la 3.0.0.\n\n"+
		"### Added\n\n- **Se agrega algo real de la prueba.** x\n\n"+
		"## [1.0.0] - 2026-01-01\n\n"+
		"- Suelta de la 1.0.0: no hereda el Added de la 2.0.0.\n\n"+
		"### Highlights\n\n- Idea de la 1.0.0.\n")
	if len(secs) != 3 {
		t.Fatalf("esperaba 3 versiones y salieron %d", len(secs))
	}
	if want := []string{"Idea de la 3.0.0."}; !reflect.DeepEqual(secs[0].Destacado, want) || len(secs[0].Titulares) != 0 || len(secs[0].Highlights) != 0 {
		t.Errorf("3.0.0: Destacado %q, Highlights %q, titulares %q; esperaba sólo el Destacado %q", secs[0].Destacado, secs[0].Highlights, secs[0].Titulares, want)
	}
	if want := map[string][]string{"Added": {"Se agrega algo real de la prueba"}}; len(secs[1].Destacado) != 0 || len(secs[1].Highlights) != 0 || !reflect.DeepEqual(secs[1].Titulares, want) {
		t.Errorf("2.0.0: Destacado %q, Highlights %q, titulares %q; esperaba sólo %q (la suelta no hereda el Destacado de la 3.0.0)", secs[1].Destacado, secs[1].Highlights, secs[1].Titulares, want)
	}
	if want := []string{"Idea de la 1.0.0."}; !reflect.DeepEqual(secs[2].Highlights, want) || len(secs[2].Destacado) != 0 || len(secs[2].Titulares) != 0 {
		t.Errorf("1.0.0: Destacado %q, Highlights %q, titulares %q; esperaba sólo los Highlights %q (la suelta no hereda el Added de la 2.0.0)", secs[2].Destacado, secs[2].Highlights, secs[2].Titulares, want)
	}
}

func TestElCuerpoDeLaReleaseLlevaLoCuradoPrimeroYUnaSolaVez(t *testing.T) {
	secs := parsear(t, changelogCurado)
	if got := cuerpoRelease(*buscar(secs, "3.0.0"), textosEs, "o/r"); got != cuerpoEsCurado3 {
		t.Errorf("cuerpo en español:\n%s\nesperaba:\n%s", got, cuerpoEsCurado3)
	}
	if got := cuerpoRelease(*buscar(secs, "3.0.0"), textosEn, "o/r"); got != cuerpoEnCurado3 {
		t.Errorf("cuerpo en inglés:\n%s\nesperaba:\n%s", got, cuerpoEnCurado3)
	}
	// El español no muestra los Highlights ni el inglés el Destacado de una versión que los tiene.
	if es := cuerpoRelease(*buscar(secs, "3.0.0"), textosEs, "o/r"); strings.Contains(es, "Highlights") || strings.Contains(es, "First curated") {
		t.Errorf("el cuerpo en español trae los Highlights:\n%s", es)
	}
	if en := cuerpoRelease(*buscar(secs, "3.0.0"), textosEn, "o/r"); strings.Contains(en, "Destacado") || strings.Contains(en, "Primera idea curada") {
		t.Errorf("el cuerpo en inglés trae el Destacado cuando hay Highlights:\n%s", en)
	}

	// Sin Highlights, el inglés cae al Destacado (la única curación que hay) con su rótulo en inglés.
	wantEn2 := "### Highlights\n\n- Única idea curada de la dos, sin versión en inglés.\n\n" +
		"### Added\n\n- Se agrega la novedad de la dos\n\n" +
		"Full details for this release are in the [CHANGELOG](https://github.com/o/r/blob/v2.0.0/CHANGELOG.md#200---2026-02-04) (written in Spanish).\n"
	if got := cuerpoRelease(*buscar(secs, "2.0.0"), textosEn, "o/r"); got != wantEn2 {
		t.Errorf("cuerpo en inglés de la 2.0.0:\n%s\nesperaba:\n%s", got, wantEn2)
	}

	// Sin ninguno, la salida es la de siempre: nada de bloque curado.
	want1 := "### Añadido\n\n- Se agrega la novedad de la uno\n\n" +
		"Detalle completo de esta versión en el [CHANGELOG](https://github.com/o/r/blob/v1.0.0/CHANGELOG.md#100---2026-01-02).\n"
	if got := cuerpoRelease(*buscar(secs, "1.0.0"), textosEs, "o/r"); got != want1 {
		t.Errorf("cuerpo de la 1.0.0:\n%s\nesperaba:\n%s", got, want1)
	}
}

func TestElReadmeEsLlevaElDestacadoYCierraConTodoLoDemas(t *testing.T) {
	secs := ordenadas(parsear(t, changelogCurado))
	got := strings.Join(bloqueReadme(secs, 3, textosEs, "o/r"), "\n")
	if got != bloqueEsCurado {
		t.Errorf("bloque en español:\n%s\nesperaba:\n%s", got, bloqueEsCurado)
	}
	// Con lo curado, el «… y N más» no aparece: lo reemplaza el cierre.
	if strings.Contains(got, "más en el") || strings.Contains(got, "… y") {
		t.Errorf("el bloque trae un «y N más» junto a lo curado:\n%s", got)
	}
}

func TestElReadmeEnLlevaHighlightsOElDestacadoOElFallback(t *testing.T) {
	secs := ordenadas(parsear(t, changelogCurado))
	got := strings.Join(bloqueReadme(secs, 3, textosEn, "o/r"), "\n")
	if got != bloqueEnCurado {
		t.Errorf("bloque en inglés:\n%s\nesperaba:\n%s", got, bloqueEnCurado)
	}
	// La nota de que el CHANGELOG está en español es del bloque inglés, tenga lo que tenga, y sólo de él.
	if !strings.HasSuffix(got, "_The changelog is written in Spanish._") {
		t.Errorf("el bloque en inglés perdió la nota final:\n%s", got)
	}
	if es := strings.Join(bloqueReadme(secs, 3, textosEs, "o/r"), "\n"); strings.Contains(es, "_The changelog") {
		t.Errorf("el bloque en español trae la nota en inglés:\n%s", es)
	}

	// Una versión con SÓLO Highlights: el inglés los muestra; el español no los usa y cae en sus
	// titulares, que están en español.
	solo := parsear(t, "## [1.0.0] - 2026-01-02\n\n### Added\n\n- **Se agrega la novedad real de la uno.** x\n\n### Highlights\n\n- Only in English.\n")
	en := strings.Join(bloqueReadme(solo, 1, textosEn, "o/r"), "\n")
	if !strings.Contains(en, "- Only in English.") || !strings.Contains(en, "And everything else, in the [CHANGELOG](") || strings.Contains(en, "Se agrega la novedad real") {
		t.Errorf("el inglés con sólo Highlights:\n%s", en)
	}
	es := strings.Join(bloqueReadme(solo, 1, textosEs, "o/r"), "\n")
	if strings.Contains(es, "Only in English") || strings.Contains(es, "todo lo demás") || !strings.Contains(es, "- Se agrega la novedad real de la uno") {
		t.Errorf("el español con sólo Highlights tenía que caer en el fallback:\n%s", es)
	}
	if cuerpo := cuerpoRelease(solo[0], textosEs, "o/r"); strings.Contains(cuerpo, "Only in English") || strings.Contains(cuerpo, "### Destacado") {
		t.Errorf("el cuerpo en español trae lo que no es suyo:\n%s", cuerpo)
	}

	// Sin ninguno de los dos: los titulares de siempre, con su «y N más».
	ninguno := parsear(t, "## [1.0.0] - 2026-01-02\n\n### Added\n\n"+vinetas("Añadido", 5))
	if got := strings.Join(bloqueReadme(ninguno, 1, textosEn, "o/r"), "\n"); !strings.Contains(got, "… and 1 more in the [CHANGELOG](") || strings.Contains(got, "everything else") {
		t.Errorf("el inglés sin nada curado tenía que dar el fallback:\n%s", got)
	}
}

func TestUnaVinetaCuradaLargaSaleEnteraEnLaReleaseYElReadme(t *testing.T) {
	// Lo curado se publica COMPLETO: una viñeta de 326 runas, que como titular saldría cortada a 160
	// con «…», sale entera y con su punto final en el cuerpo y en el README, en los dos idiomas. El
	// tope de lo curado es de CUÁNTAS viñetas (6 y 4), no de cuánto texto lleva cada una.
	larga := strings.TrimSpace(strings.Repeat("palabra ", 40)) + " final."
	if utf8.RuneCountInString(larga) <= topeTitular+1 {
		t.Fatal("la propia prueba no sirve: la viñeta larga entra en el tope de un titular")
	}
	secs := parsear(t, "## [1.0.0] - 2026-01-02\n\n### Destacado\n\n- "+larga+"\n\n### Highlights\n\n- "+larga+"\n")
	for _, c := range []struct {
		nombre string
		t      textos
	}{{"es", textosEs}, {"en", textosEn}} {
		if cuerpo := cuerpoRelease(secs[0], c.t, "o/r"); !strings.Contains(cuerpo, "- "+larga+"\n") || strings.Contains(cuerpo, "…") {
			t.Errorf("%s: el cuerpo no trae la viñeta curada entera:\n%s", c.nombre, cuerpo)
		}
		if bloque := strings.Join(bloqueReadme(secs, 1, c.t, "o/r"), "\n"); !strings.Contains(bloque, "- "+larga+"\n") || strings.Contains(bloque, "…") {
			t.Errorf("%s: el README no trae la viñeta curada entera:\n%s", c.nombre, bloque)
		}
	}
}

func TestLosTopesDeLoCurado(t *testing.T) {
	// Siete viñetas curadas en cada idioma: el cuerpo muestra 6, el README 4, y lo que sobra se
	// corta sin más —no hay «y N más» para lo curado—: el cierre manda al CHANGELOG.
	secs := parsear(t, "## [1.0.0] - 2026-01-02\n\n### Destacado\n\n"+curadasN("Idea", 7)+
		"\n### Highlights\n\n"+curadasN("Idea inglesa", 7)+
		"\n### Added\n\n- **Se agrega la novedad real de la uno.** x\n")
	s := secs[0]
	if len(s.Destacado) != 7 || len(s.Highlights) != 7 {
		t.Fatalf("el parser tenía que guardar las 7 y el tope es de quien muestra: %d / %d", len(s.Destacado), len(s.Highlights))
	}
	for _, c := range []struct {
		nombre, prefijo string
		t               textos
	}{{"es", "Idea", textosEs}, {"en", "Idea inglesa", textosEn}} {
		cuerpo := cuerpoRelease(s, c.t, "o/r")
		if n := strings.Count(cuerpo, "número"); n != topeRelease || !strings.Contains(cuerpo, c.prefijo+" número 6 de la prueba.") || strings.Contains(cuerpo, "número 7") {
			t.Errorf("%s: el cuerpo tenía que mostrar %d viñetas curadas y no la séptima (muestra %d):\n%s", c.nombre, topeRelease, n, cuerpo)
		}
		if strings.Contains(cuerpo, "… y") || strings.Contains(cuerpo, "… and") {
			t.Errorf("%s: lo curado no se anuncia con un «y N más»:\n%s", c.nombre, cuerpo)
		}
		bloque := strings.Join(bloqueReadme(secs, 1, c.t, "o/r"), "\n")
		if n := strings.Count(bloque, "número"); n != topeReadme || !strings.Contains(bloque, c.prefijo+" número 4 de la prueba.") || strings.Contains(bloque, "número 5") {
			t.Errorf("%s: el README tenía que mostrar %d viñetas curadas y no la quinta (muestra %d):\n%s", c.nombre, topeReadme, n, bloque)
		}
		if strings.Contains(bloque, "… y") || strings.Contains(bloque, "… and") || !strings.Contains(bloque, "[CHANGELOG](") {
			t.Errorf("%s: el README tenía que cerrar con el enlace y sin «y N más»:\n%s", c.nombre, bloque)
		}
	}
}

func TestLoCuradoNoCuentaEnElYNMas(t *testing.T) {
	// El cuerpo: 8 titulares en Añadido dan «y 2 más» (8 - 6), con tres viñetas curadas de por medio.
	secs := parsear(t, versionDePrueba("1.0.0", "2026-01-02", 8, 0, 0, 0, 0, 0)+"\n### Destacado\n\n"+curadasN("Idea", 3))
	cuerpo := cuerpoRelease(secs[0], textosEs, "o/r")
	if !strings.Contains(cuerpo, "- … y 2 más\n") || strings.Count(cuerpo, "más") != 1 {
		t.Errorf("el «y N más» de Añadido tenía que ser 2 (8 titulares, tope 6) sin sumar lo curado:\n%s", cuerpo)
	}
	if n := strings.Count(cuerpo, "Idea número"); n != 3 {
		t.Errorf("las 3 viñetas curadas tenían que salir una vez cada una (salen %d):\n%s", n, cuerpo)
	}

	// El README del idioma que NO tiene curación para la versión (español con sólo Highlights): el
	// «y N más» cuenta sólo los titulares derivados, 6 - 4 = 2, y no los Highlights.
	secs = parsear(t, versionDePrueba("1.0.0", "2026-01-02", 6, 0, 0, 0, 0, 0)+"\n### Highlights\n\n"+curadasN("Idea inglesa", 3))
	if bloque := strings.Join(bloqueReadme(secs, 1, textosEs, "o/r"), "\n"); !strings.Contains(bloque, "… y 2 más en el [CHANGELOG](") {
		t.Errorf("el «y N más» del español tenía que ser 2 sin sumar los Highlights:\n%s", bloque)
	}
	if got := len(enOrden(secs[0])); got != 6 {
		t.Errorf("enOrden devolvió %d titulares; tenían que ser los 6 derivados y ni uno curado", got)
	}

	// Y con los DOS grupos curados a la vez, enOrden sigue siendo sólo lo derivado: ni el Destacado
	// ni los Highlights se cuelan en la lista que alimenta el «y N más». Con curación en el idioma
	// el README ni siquiera llama a enOrden, así que sólo una llamada directa lo ve.
	secs = parsear(t, versionDePrueba("1.0.0", "2026-01-02", 6, 0, 0, 0, 0, 0)+
		"\n### Destacado\n\n"+curadasN("Idea", 3)+"\n### Highlights\n\n"+curadasN("Idea inglesa", 3))
	if got := len(enOrden(secs[0])); got != 6 {
		t.Errorf("enOrden devolvió %d titulares con Destacado y Highlights presentes; tenían que ser los 6 derivados", got)
	}
}

func TestLoCuradoNeutralizaElConteoHistoricoDeHerramientas(t *testing.T) {
	secs := parsear(t, "## [1.0.0] - 2026-01-02\n\n### Destacado\n\n"+
		"- Pasamos de 43 a 58 herramientas en el catálogo, y hoy hay **80** tools más y 7 TOOLS.\n"+
		"- Se documentan 3 herramientas.\n\n"+
		"### Highlights\n\n- We went from 43 to 58 tools in the catalog, and 3 herramientas too.\n")
	// Las mismas dos expresiones de internal/mcp/readme_toolcount_test.go, una por README.
	guardas := []*regexp.Regexp{
		regexp.MustCompile(`\*\*(\d+)\s+herramientas\*\*|(\d+)\s+herramientas`),
		regexp.MustCompile(`\*\*(\d+)\s+tools\*\*|(\d+)\s+tools`),
	}
	salidas := []string{
		cuerpoRelease(secs[0], textosEs, "o/r"), cuerpoRelease(secs[0], textosEn, "o/r"),
		strings.Join(bloqueReadme(secs, 1, textosEs, "o/r"), "\n"), strings.Join(bloqueReadme(secs, 1, textosEn, "o/r"), "\n"),
	}
	for _, salida := range salidas {
		for _, re := range guardas {
			if m := re.FindString(salida); m != "" {
				t.Errorf("una viñeta curada trae %q, que la guarda de los README leería como un conteo:\n%s", m, salida)
			}
		}
	}
	// Se ve igual: con el espacio de no separación leído como un espacio, el texto es el original.
	if got, want := strings.ReplaceAll(secs[0].Destacado[0], nbsp, " "), "Pasamos de 43 a 58 herramientas en el catálogo, y hoy hay 80 tools más y 7 TOOLS."; got != want {
		t.Errorf("la viñeta cambió de texto: %q; esperaba %q", got, want)
	}
}

func TestUnaVinetaCuradaConMarkdownHostilSaleLimpia(t *testing.T) {
	secs := parsear(t, "## [1.0.0] - 2026-01-02\n\n### Destacado\n\n"+
		"- Una tabla | con barras | y <script>alert(1)</script> afuera, pero ´<tag> | x´ adentro.\n"+
		"- Un backtick ´suelto y **negrita sin cerrar y [enlace roto(sin cierre.\n"+
		"- <!-- novedades:fin --> en prosa no es un marcador.\n"+
		"- Código con ´´dos `backticks` adentro´´ y un [enlace con ´[corchetes]´](https://example.com/x).\n")
	want := []string{
		"Una tabla | con barras | y &lt;script>alert(1)&lt;/script> afuera, pero `<tag> | x` adentro.",
		"Un backtick suelto y negrita sin cerrar y [enlace roto(sin cierre.",
		"&lt;!-- novedades:fin --> en prosa no es un marcador.",
		"Código con ``dos `backticks` adentro`` y un enlace con `[corchetes]`.",
	}
	if got := secs[0].Destacado; !reflect.DeepEqual(got, want) {
		t.Fatalf("Destacado:\n%q\nesperaba:\n%q", got, want)
	}
	for _, x := range secs[0].Destacado {
		if p := problemaDeTexto(x); p != "" {
			t.Errorf("la viñeta %q %s", x, p)
		}
	}
	// Y un marcador mencionado en prosa dentro de una viñeta no desarma el README: sigue habiendo
	// un solo par, y volver a generar da lo mismo.
	uno, err := reemplazarBloque(readmeBase, bloqueReadme(secs, 1, textosEs, "o/r"))
	if err != nil {
		t.Fatal(err)
	}
	dos, err := reemplazarBloque(uno, bloqueReadme(secs, 1, textosEs, "o/r"))
	if err != nil || dos != uno {
		t.Errorf("volver a generar cambió el README (err=%v):\n%s", err, primeraDiferencia(dos, uno))
	}
}

func TestCRLFDaLoMismoQueLFTambienEnLoCurado(t *testing.T) {
	lf := parsear(t, changelogCurado)
	crlf := parsear(t, strings.ReplaceAll(changelogCurado, "\n", "\r\n"))
	if !reflect.DeepEqual(lf, crlf) {
		t.Errorf("el mismo CHANGELOG con CRLF dio otra cosa:\nLF:   %+v\nCRLF: %+v", lf, crlf)
	}
	multi := parsear(t, "## [1.0.0] - 2026-01-02\r\n\r\n### Destacado\r\n\r\n- Una viñeta\r\n  en dos líneas.\r\n")
	if want := []string{"Una viñeta en dos líneas."}; !reflect.DeepEqual(multi[0].Destacado, want) {
		t.Errorf("Destacado con CRLF = %q; esperaba %q", multi[0].Destacado, want)
	}
}

func TestElComandoPublicaLoCuradoDePuntaAPunta(t *testing.T) {
	dir := t.TempDir()
	cl := escribir(t, dir, "CHANGELOG.md", changelogCurado)
	es := escribir(t, dir, "README.md", readmeBase)
	en := escribir(t, dir, "README.en.md", readmeBase)

	codigo, _, errOut := correr(t, "-readme", "-changelog", cl, "-root", dir, "-repo", "o/r")
	if codigo != 0 || errOut != "" {
		t.Fatalf("-readme: código %d, stderr %q", codigo, errOut)
	}
	if got, want := leer(t, es), readmeCon(bloqueEsCurado); got != want {
		t.Errorf("README.md (%s):\n%s", primeraDiferencia(got, want), got)
	}
	if got, want := leer(t, en), readmeCon(bloqueEnCurado); got != want {
		t.Errorf("README.en.md (%s):\n%s", primeraDiferencia(got, want), got)
	}
	// Idempotente también con lo curado.
	if codigo, out, _ := correr(t, "-readme", "-changelog", cl, "-root", dir, "-repo", "o/r"); codigo != 0 || strings.Count(out, "sin cambios") != 2 {
		t.Errorf("la segunda corrida tenía que dar «sin cambios» dos veces: código %d\n%s", codigo, out)
	}

	for _, c := range []struct {
		args []string
		want string
	}{
		{[]string{"-version", "3.0.0"}, cuerpoEsCurado3},
		{[]string{"-version", "v3.0.0", "-lang", "en"}, cuerpoEnCurado3},
	} {
		codigo, out, errOut := correr(t, append([]string{"-changelog", cl, "-repo", "o/r"}, c.args...)...)
		if codigo != 0 || errOut != "" || out != c.want {
			t.Errorf("%v: código %d, stderr %q, stdout:\n%s\nesperaba:\n%s", c.args, codigo, errOut, out, c.want)
		}
	}
}

// ---- entradas al azar -----------------------------------------------------------------------

// fichasDeMarkdown son los trozos con los que se arma la basura al azar: tiras de backticks, `**`,
// enlaces, `<`, puntos, palabras larguísimas y los conteos que la guarda de los README vigila.
var fichasDeMarkdown = []string{
	"´", "´", "´´", "´´´", "**", "**", "*", "[", "]", "(", ")", "[txt](url)", "[a ´b´](c)",
	"<", "<tag>", ".", ". ", ": ", ", ", "; ", " ", " ", "  ", "\t",
	"a", "bc", "palabra", "otra", "supercalifragilistico", "ñandú", "é", "🔴", "→", "…",
	"p. ej. ", "ej. ", "vs. ", "1", "42 herramientas", "7 tools", "5 TOOLS",
	"-", "_", "#", `"`, "«", "»", "|", "&", "&lt;",
	strings.Repeat("x", 90), strings.Repeat("<", 90),
}

// entradaAlAzar arma el texto de una viñeta de mentira con esas fichas.
func entradaAlAzar(rng *rand.Rand) string {
	var b strings.Builder
	if rng.Intn(3) == 0 {
		b.WriteString("**")
	}
	for n := 1 + rng.Intn(80); n > 0; n-- {
		b.WriteString(md(fichasDeMarkdown[rng.Intn(len(fichasDeMarkdown))]))
	}
	return b.String()
}

// TestTitularNuncaRompeSusPromesas tira basura con la forma del Markdown del CHANGELOG —tiras de
// backticks, `**`, enlaces, `<`, puntos, palabras larguísimas— y comprueba que, pase lo que pase,
// lo que sale cumple todo lo que el generador promete. La semilla es fija: un rojo se reproduce.
func TestTitularNuncaRompeSusPromesas(t *testing.T) {
	rng := rand.New(rand.NewSource(20261003))
	for i := 0; i < 40000; i++ {
		entrada := entradaAlAzar(rng)
		salida := titular(entrada)
		if salida == "" {
			continue
		}
		if p := problemaDeTitular(salida); p != "" {
			t.Fatalf("entrada n.º %d %q\nsalida %q\n%s", i, entrada, salida, p)
		}
		// Un `<` escapado nunca queda a medias, salvo que el corte lo haya partido (y entonces el
		// titular termina en «…»).
		if !strings.HasSuffix(salida, "…") && strings.Count(salida, "&lt") != strings.Count(salida, "&lt;") {
			t.Fatalf("entrada n.º %d %q\nsalida %q\nun «&lt;» quedó sin su punto y coma", i, entrada, salida)
		}
	}
}

// TestCompletaNuncaRompeSusPromesas es el mismo fuzz para las viñetas curadas. Lo que sale es una
// sola línea limpia, sin HTML suelto, con el código cerrado y sin conteos a la vista, pase lo que
// pase con la entrada; y como NO hay tope, un `<` escapado nunca queda a medias, ni al final. Cuenta
// también las salidas más largas que un titular: si ninguna lo fuera, la prueba no estaría
// mirando que la viñeta curada sale completa.
func TestCompletaNuncaRompeSusPromesas(t *testing.T) {
	rng := rand.New(rand.NewSource(20261003))
	largas := 0
	for i := 0; i < 40000; i++ {
		entrada := entradaAlAzar(rng)
		salida := completa(entrada)
		if salida == "" {
			continue
		}
		if p := problemaDeTexto(salida); p != "" {
			t.Fatalf("entrada n.º %d %q\nsalida %q\n%s", i, entrada, salida, p)
		}
		if strings.Count(salida, "&lt") != strings.Count(salida, "&lt;") {
			t.Fatalf("entrada n.º %d %q\nsalida %q\nun «&lt;» quedó sin su punto y coma", i, entrada, salida)
		}
		if utf8.RuneCountInString(salida) > topeTitular+1 {
			largas++
		}
	}
	if largas == 0 {
		t.Error("ninguna salida pasó del tope de un titular: la prueba no verifica que lo curado sale completo")
	}
}

// ---- contra los archivos reales del repo ----------------------------------------------------

// TestChangelogRealTieneLaEstructuraEsperada es el humo del CHANGELOG verdadero. Mira ESTRUCTURA y
// no texto: el archivo crece con cada release, y una prueba que fijara un titular concreto se
// pondría roja por trabajo bueno.
func TestChangelogRealTieneLaEstructuraEsperada(t *testing.T) {
	raiz := raizDelRepo(t)
	secs, err := parsearChangelog(leer(t, filepath.Join(raiz, "CHANGELOG.md")))
	if err != nil {
		t.Fatalf("CHANGELOG.md: %v", err)
	}
	if len(secs) < 100 {
		t.Fatalf("el CHANGELOG real da %d versiones y tienen que ser 100 o más: ¿el parser dejó de leerlo bien?", len(secs))
	}
	for i, s := range secs {
		if i > 0 && compararVersiones(secs[i-1].Version, s.Version) <= 0 {
			t.Errorf("CHANGELOG.md:%d: la %s no es menor que la anterior del archivo (%s, línea %d): el orden tiene que ser descendente",
				s.Linea, s.Version, secs[i-1].Version, secs[i-1].Linea)
		}
		if _, err := time.Parse("2006-01-02", s.Fecha); err != nil {
			t.Errorf("CHANGELOG.md:%d: la %s no tiene una fecha AAAA-MM-DD legible en el encabezado (%q)", s.Linea, s.Version, s.Fecha)
		}
		if s.Ancla == "" {
			t.Errorf("CHANGELOG.md:%d: la %s quedó sin ancla", s.Linea, s.Version)
		}
		titulares := enOrden(s)
		if len(titulares) == 0 {
			t.Errorf("CHANGELOG.md:%d: la %s no tiene ni un titular", s.Linea, s.Version)
		}
		for _, h := range titulares {
			if p := problemaDeTitular(h); p != "" {
				t.Errorf("CHANGELOG.md:%d (v%s): el titular %q %s", s.Linea, s.Version, h, p)
			}
		}
		// Si la versión trae `### Destacado` o `### Highlights`, lo que se publica de ellos cumple lo
		// mismo que un titular salvo el tope y el signo final: son oraciones completas.
		for _, curada := range append(append([]string(nil), s.Destacado...), s.Highlights...) {
			if p := problemaDeTexto(curada); p != "" {
				t.Errorf("CHANGELOG.md:%d (v%s): la viñeta curada %q %s", s.Linea, s.Version, curada, p)
			}
		}
		// Y lo que se publica con ellos: sólo encabezados, viñetas, líneas en blanco y el cierre.
		cuerpo := cuerpoRelease(s, textosEs, repoPorDefecto)
		lineas := strings.Split(strings.TrimSuffix(cuerpo, "\n"), "\n")
		if cierre := lineas[len(lineas)-1]; !strings.HasPrefix(cierre, "Detalle completo de esta versión en el [CHANGELOG](") || !strings.HasSuffix(cierre, "#"+s.Ancla+").") {
			t.Errorf("v%s: el cuerpo de la release no cierra con el enlace a su sección: %q", s.Version, cierre)
		}
		for _, l := range lineas[:len(lineas)-1] {
			if l != "" && !strings.HasPrefix(l, "### ") && !strings.HasPrefix(l, "- ") {
				t.Errorf("v%s: línea inesperada en el cuerpo de la release: %q", s.Version, l)
			}
		}
	}

	// El comando entero, contra el archivo real y sin tocar nada.
	codigo, out, errOut := correr(t, "-changelog", filepath.Join(raiz, "CHANGELOG.md"), "-version", secs[0].Version, "-estricto")
	if codigo != 0 || errOut != "" || !strings.HasPrefix(out, "### ") {
		t.Errorf("-version %s -estricto: código %d, stderr %q", secs[0].Version, codigo, errOut)
	}
}

// TestReadmeNovedadesAlDia es la guarda: lo que hay entre los marcadores de los dos README tiene
// que ser EXACTAMENTE lo que el generador escribiría hoy desde el CHANGELOG. Sin ella el bloque
// se quedaba en la versión de la semana en que alguien lo regeneró.
func TestReadmeNovedadesAlDia(t *testing.T) {
	raiz := raizDelRepo(t)
	secs, err := parsearChangelog(leer(t, filepath.Join(raiz, "CHANGELOG.md")))
	if err != nil {
		t.Fatalf("CHANGELOG.md: %v", err)
	}
	secs = ordenadas(secs)
	for _, r := range []struct {
		nombre string
		t      textos
	}{{"README.md", textosEs}, {"README.en.md", textosEn}} {
		t.Run(r.nombre, func(t *testing.T) {
			actual := leer(t, filepath.Join(raiz, r.nombre))
			esperado, err := reemplazarBloque(actual, bloqueReadme(secs, versionesPorDefecto, r.t, repoPorDefecto))
			if err != nil {
				t.Fatalf("%s: %v\nRegeneralo con: go run ./deploy/cmd/notas-release -readme", r.nombre, err)
			}
			if esperado != actual {
				t.Errorf("%s: lo que hay entre %s y %s no coincide con el CHANGELOG.\n"+
					"Regeneralo con: go run ./deploy/cmd/notas-release -readme\n"+
					"Primera línea que difiere, %s",
					r.nombre, marcaInicio, marcaFin, primeraDiferencia(actual, esperado))
			}
		})
	}
}

// TestSinPublicarDeLosReadmeReales aplica la guarda de `base=` a los README verdaderos: si alguno
// tiene el bloque «sin publicar» y su base no es la última versión del CHANGELOG, falla. Sin
// bloque, pasa.
func TestSinPublicarDeLosReadmeReales(t *testing.T) {
	raiz := raizDelRepo(t)
	secs, err := parsearChangelog(leer(t, filepath.Join(raiz, "CHANGELOG.md")))
	if err != nil {
		t.Fatalf("CHANGELOG.md: %v", err)
	}
	ultima := ordenadas(secs)[0].Version
	for _, nombre := range []string{"README.md", "README.en.md"} {
		if err := comprobarSinPublicar(leer(t, filepath.Join(raiz, nombre)), nombre, ultima); err != nil {
			t.Error(err)
		}
	}
}
