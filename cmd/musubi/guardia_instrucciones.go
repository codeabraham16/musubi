package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"
	"unicode"

	"musubi/internal/memory"
)

// ── La guardia de instrucciones ──────────────────────────────────────────────────────────────
//
// POR QUÉ EXISTE. Claude Code carga en CADA sesión —y otra vez después de cada compactación— el
// CLAUDE.md del proyecto, su CLAUDE.local.md y su .claude/CLAUDE.md, los CLAUDE.md de las carpetas
// de arriba, el ~/.claude/CLAUDE.md y el MEMORY.md de la memoria automática, con todo lo que
// importen por @ruta. Nada de eso avisa cuando crece. Medido el 2026-10-14: el CLAUDE.md de
// altura-erp pesaba 79.196 caracteres (499 líneas) y entraba entero en cada sesión. Partido en un
// núcleo de 199 líneas más 6 reglas .claude/rules/*.md con `paths:`, esas reglas cargan sólo cuando
// se lee o se edita un archivo que matchea (probado en vivo). La guardia está para que eso no vuelva
// a crecer sin que nadie lo note: avisa al arrancar, una vez por día, y nombra los archivos con sus
// números.
//
// LOS TOPES SON LOS DE CLAUDE CODE, no inventados acá. La doc (code.claude.com/docs/en/memory) pide
// menos de 200 líneas por CLAUDE.md, y del MEMORY.md carga «las primeras 200 líneas o los primeros
// 25KB, lo que llegue antes»: lo que pasa de ahí NO llega al modelo. Y se mide COMO lo mide Claude
// Code, leído del binario 2.1.299: `XF=200,Zse=25000`, y `$ut(t)` recorta el texto (trim), cuenta los
// saltos de línea más uno y toma `r.length`, que son unidades UTF-16 de JavaScript aunque el código
// las llame byteCount. O sea: 25 KB son 25.000 y no 25.600; un CRLF da las mismas líneas que un LF
// (y suma su `\r` al tamaño, igual que allá); y los renglones en blanco del final no cuentan. Medir
// bytes de disco daba de más con cada tilde, y la guardia habría dicho «no se carga» de algo que sí
// se carga.

// topeLineasDeInstrucciones es el máximo de líneas: el que recomienda la doc para un CLAUDE.md y
// el corte de Claude Code para el MEMORY.md. Avisa con MÁS de esto: 200 justas no avisan.
const topeLineasDeInstrucciones = 200

// topeTamanoDeInstrucciones es el corte de tamaño del MEMORY.md (`Zse=25000` en Claude Code, «25KB»
// en la doc), en las unidades de medirTexto. Avisa con MÁS de esto.
const topeTamanoDeInstrucciones = 25000

// saltosDeImportMaximos es la profundidad de los @imports que Claude Code sigue: «a maximum depth
// of four hops» (doc).
const saltosDeImportMaximos = 4

// metaGuardiaDeInstrucciones marca el día en que la guardia avisó en este proyecto (la meta es de la
// base del proyecto). Se escribe sólo cuando el aviso sale: un proyecto sano no gasta la marca, y
// si mañana un archivo pasa el tope, avisa mañana.
const metaGuardiaDeInstrucciones = "arranque_guardia_instrucciones"

// surfaceGuardiaDeInstrucciones es la superficie del ledger de la línea que va al agente.
const surfaceGuardiaDeInstrucciones = "startup_guardia_instrucciones"

// archivoDeInstrucciones es un archivo que Claude Code carga al arrancar: dónde está y cómo se lo
// nombra en el aviso. esMemoria marca el MEMORY.md, que Claude Code CORTA en el tope (los demás
// entran enteros) y que no expande @imports.
type archivoDeInstrucciones struct {
	etiqueta  string
	ruta      string
	esMemoria bool
}

// medidaDeInstrucciones es lo que pesa un archivo, con lo que importa sumado: líneas y tamaño en
// las unidades de medirTexto, y cuántos archivos importados entraron en la cuenta.
type medidaDeInstrucciones struct {
	archivoDeInstrucciones
	lineas     int
	tamano     int
	importados int
}

func (m medidaDeInstrucciones) pasaElTope() bool {
	return m.lineas > topeLineasDeInstrucciones || m.tamano > topeTamanoDeInstrucciones
}

func (m medidaDeInstrucciones) describir() string {
	nombre := m.etiqueta
	if m.importados > 0 {
		nombre += " con sus @imports"
	}
	lineas := "líneas"
	if m.lineas == 1 {
		lineas = "línea"
	}
	return fmt.Sprintf("%s (%d %s, %s)", nombre, m.lineas, lineas, kilos(m.tamano))
}

// avisoDeInstrucciones son las dos líneas del aviso: la del dueño va como systemMessage (la ve la
// persona, no el modelo) y la del agente como additionalContext. Vacías = nada que decir.
type avisoDeInstrucciones struct {
	dueno  string
	agente string
}

// sumarGuardiaDeInstrucciones es el enganche del arranque (detectOutputSegunFuente): suma el aviso
// al envelope que el arranque ya armó, o arma uno si el arranque iba a callar. Sólo corre en la rama
// del arranque, nunca en la de compactación: a mitad de sesión no hay nada que hacer con esto, y el
// aviso del día se gastaría donde nadie lo puede atender.
func sumarGuardiaDeInstrucciones(out string, store startupStore, root, sessionID string) string {
	dirConfig, _ := dirConfigDeClaude() // sin HOME ni CLAUDE_CONFIG_DIR: sólo los del proyecto
	home, _ := os.UserHomeDir()
	return sumarGuardiaDeInstruccionesCon(out, store, sessionID, root, dirConfig, home, time.Now())
}

// sumarGuardiaDeInstruccionesCon es sumarGuardiaDeInstrucciones con la carpeta de configuración de
// Claude Code, el HOME y la hora inyectados, para probarla sin depender de la PC que la corre.
//
// Sin memoria (store nil) calla: sin la marca no hay forma de avisar UNA vez por día, y un aviso en
// cada arranque es ruido que se aprende a ignorar.
func sumarGuardiaDeInstruccionesCon(out string, store startupStore, sessionID, root, dirConfig, home string, ahora time.Time) string {
	if store == nil {
		return out
	}
	dia := ahora.Format("2006-01-02")
	if v, ok, _ := store.GetMeta(metaGuardiaDeInstrucciones); ok && v == dia {
		return out
	}
	aviso := armarAvisoDeInstrucciones(medirInstrucciones(root, dirConfig, home))
	con, ok := conAvisoDeInstrucciones(out, aviso)
	if !ok {
		return out
	}
	// Se marca y se cobra DESPUÉS de saber que el envelope sale (la regla de assembleAccounted).
	_ = store.SetMeta(metaGuardiaDeInstrucciones, dia)
	_, _ = store.LedgerAdd(sessionID, surfaceGuardiaDeInstrucciones, memory.EstimateTokens(aviso.agente))
	return con
}

// conAvisoDeInstrucciones mete el aviso en el envelope del SessionStart: la línea del agente se
// suma al additionalContext y la del dueño va como systemMessage, un campo de primer nivel que
// assembleHookContext no arma. Si el arranque iba a callar, el envelope se arma por la MISMA puerta
// que el resto, la de la lista blanca de eventos (ver hook_contrato_test.go). Devuelve false si no
// hay aviso o si el envelope no se pudo armar: entonces no se marca el día ni se cobra.
func conAvisoDeInstrucciones(out string, aviso avisoDeInstrucciones) (string, bool) {
	if aviso.agente == "" || aviso.dueno == "" {
		return out, false
	}
	base := out
	if base == "" {
		if base = assembleHookContext("SessionStart", aviso.agente); base == "" {
			return out, false
		}
	}
	var env map[string]interface{}
	if err := json.Unmarshal([]byte(base), &env); err != nil {
		return out, false
	}
	especifico, _ := env["hookSpecificOutput"].(map[string]interface{})
	if especifico == nil {
		return out, false
	}
	if out != "" {
		previo, _ := especifico["additionalContext"].(string)
		especifico["additionalContext"] = strings.TrimSpace(previo + "\n\n" + aviso.agente)
	}
	// Si otro bloque ya trae un systemMessage, se suma: no se pisa.
	mensaje := aviso.dueno
	if previo, _ := env["systemMessage"].(string); previo != "" {
		mensaje = previo + "\n" + aviso.dueno
	}
	env["systemMessage"] = mensaje
	datos, err := json.MarshalIndent(env, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "musubi detect: no se pudo sumar la guardia de instrucciones: %v\n", err)
		return out, false
	}
	return string(datos), true
}

// armarAvisoDeInstrucciones arma las dos líneas con los archivos que pasan el tope. El MEMORY.md
// lleva su propia frase, porque a él le pasa otra cosa: no se carga entero de más, se CORTA.
func armarAvisoDeInstrucciones(medidas []medidaDeInstrucciones) avisoDeInstrucciones {
	var enteros, cortados []string
	for _, m := range medidas {
		if !m.pasaElTope() {
			continue
		}
		if m.esMemoria {
			cortados = append(cortados, m.describir())
		} else {
			enteros = append(enteros, m.describir())
		}
	}
	if len(enteros) == 0 && len(cortados) == 0 {
		return avisoDeInstrucciones{}
	}
	const tope = "200 líneas o 25 KB"
	var dueno, agente []string
	if len(enteros) > 0 {
		pasa, carga, lo := "pasa", "se carga entero", "lo"
		if len(enteros) > 1 {
			pasa, carga, lo = "pasan", "se cargan enteros", "los"
		}
		lista := enumerar(enteros)
		dueno = append(dueno, fmt.Sprintf("%s %s lo recomendado (%s) y %s en cada sesión. "+
			"Mové lo que es de un área a .claude/rules/<tema>.md con paths: en el frontmatter, que carga sólo al tocar esos archivos.",
			lista, pasa, tope, carga))
		agente = append(agente, fmt.Sprintf("%s %s lo recomendado (%s) y %s en cada sesión. "+
			"No %s reorganices por tu cuenta: si el dueño lo pide, mové lo específico de un área a .claude/rules/<tema>.md con paths: (globs) en el frontmatter.",
			lista, pasa, tope, carga, lo))
	}
	if len(cortados) > 0 {
		lista := enumerar(cortados)
		dueno = append(dueno, fmt.Sprintf("De %s Claude Code carga sólo las primeras %s: lo que pasa de ahí no llega al modelo. "+
			"Dejá ahí el índice y pasá el detalle a archivos de tema.", lista, tope))
		agente = append(agente, fmt.Sprintf("De %s sólo te llegan las primeras %s: lo que pasa de ahí no lo ves.", lista, tope))
	}
	return avisoDeInstrucciones{
		dueno:  "Musubi · guardia de instrucciones: " + strings.Join(dueno, " ") + " Te lo recuerdo una vez por día.",
		agente: "[Musubi — guardia de instrucciones] " + strings.Join(agente, " "),
	}
}

// medirInstrucciones mide cada archivo de instrucciones que existe, con sus @imports sumados. Sólo
// lee: no escribe nada, ni abre la memoria.
func medirInstrucciones(root, dirConfig, home string) []medidaDeInstrucciones {
	var medidas []medidaDeInstrucciones
	vistos := map[string]bool{}
	for _, a := range archivosDeInstrucciones(root, dirConfig, home) {
		clave := claveDeRuta(a.ruta)
		if vistos[clave] {
			continue
		}
		vistos[clave] = true
		saltos := saltosDeImportMaximos
		if a.esMemoria {
			saltos = 0 // el MEMORY.md es un índice: sus enlaces no se expanden al arrancar
		}
		lineas, tamano, importados, ok := medirConImports(a.ruta, home, saltos, map[string]bool{})
		if !ok {
			continue
		}
		medidas = append(medidas, medidaDeInstrucciones{archivoDeInstrucciones: a, lineas: lineas, tamano: tamano, importados: importados})
	}
	return medidas
}

// archivosDeInstrucciones lista los archivos que Claude Code carga al arrancar en root: los del
// proyecto, los de las carpetas de arriba (la doc: «every directory above it»), el del usuario y el
// MEMORY.md del proyecto. Puede nombrar archivos que no existen; medirInstrucciones los saltea.
func archivosDeInstrucciones(root, dirConfig, home string) []archivoDeInstrucciones {
	if abs, err := filepath.Abs(root); err == nil {
		root = abs
	}
	var lista []archivoDeInstrucciones
	for _, rel := range []string{"CLAUDE.md", "CLAUDE.local.md", filepath.Join(".claude", "CLAUDE.md")} {
		lista = append(lista, archivoDeInstrucciones{etiqueta: filepath.ToSlash(rel), ruta: filepath.Join(root, rel)})
	}
	for d, previo := filepath.Dir(root), root; d != previo; d, previo = filepath.Dir(d), d {
		for _, n := range []string{"CLAUDE.md", "CLAUDE.local.md"} {
			ruta := filepath.Join(d, n)
			lista = append(lista, archivoDeInstrucciones{etiqueta: abreviarRuta(ruta, home), ruta: ruta})
		}
	}
	if dirConfig == "" {
		return lista
	}
	usuario := filepath.Join(dirConfig, "CLAUDE.md")
	lista = append(lista, archivoDeInstrucciones{etiqueta: abreviarRuta(usuario, home), ruta: usuario})
	for _, c := range carpetasDeMemoria(filepath.Join(dirConfig, "projects"), root) {
		ruta := filepath.Join(c, "memory", "MEMORY.md")
		lista = append(lista, archivoDeInstrucciones{etiqueta: abreviarRuta(ruta, home), ruta: ruta, esMemoria: true})
	}
	return lista
}

// carpetasDeMemoria encuentra en dirProyectos (`<config>/projects`) las carpetas de la memoria
// automática de root. La doc: la carpeta sale del repo de git, así que los worktrees y las subcarpetas
// comparten la del checkout principal, y fuera de git es la raíz del proyecto; también se mira la de
// root a secas por si una versión vieja de Claude Code la derivaba del cwd. CLAUDE_CODE_PROJECT_DIR_NAME,
// si está, le gana a todo (doc, desde 2.1.234).
func carpetasDeMemoria(dirProyectos, root string) []string {
	if n := os.Getenv("CLAUDE_CODE_PROJECT_DIR_NAME"); n != "" {
		return carpetasDeMemoriaPorNombre(dirProyectos, []string{n})
	}
	return carpetasDeMemoriaPorNombre(dirProyectos, []string{carpetaDeProyecto(raizDelRepoPrincipal(root)), carpetaDeProyecto(root)})
}

// carpetasDeMemoriaPorNombre busca los nombres en dirProyectos SIN distinguir mayúsculas en la letra
// de unidad. Medido en esta máquina el 2026-10-14: VS Code la pasa en minúscula y la CLI en mayúscula
// (conviven c--Proyectos-Musubi y C--Proyectos-Proyecto-menir-altura-wt-claude-md), y buscar el
// nombre exacto encontraba la mitad. En Windows el sistema de archivos no distingue mayúsculas en
// ningún lado, así que ahí se compara el nombre entero sin distinguirlas.
func carpetasDeMemoriaPorNombre(dirProyectos string, nombres []string) []string {
	entradas, err := os.ReadDir(dirProyectos)
	if err != nil {
		return nil
	}
	var carpetas []string
	for _, e := range entradas {
		if !e.IsDir() {
			continue
		}
		for _, n := range nombres {
			if mismaCarpetaDeProyecto(e.Name(), n) {
				carpetas = append(carpetas, filepath.Join(dirProyectos, e.Name()))
				break
			}
		}
	}
	return carpetas
}

func mismaCarpetaDeProyecto(a, b string) bool {
	if runtime.GOOS == "windows" {
		return strings.EqualFold(a, b)
	}
	return a == b || (a != "" && b != "" && strings.EqualFold(a[:1], b[:1]) && a[1:] == b[1:])
}

// carpetaDeProyecto es el nombre de la carpeta de un proyecto bajo `<config>/projects`: su ruta
// absoluta con todo lo que no es alfanumérico ASCII cambiado por `-`. Claude Code lo hace sobre las
// unidades UTF-16 de la ruta, así que un carácter fuera del plano básico deja dos guiones.
func carpetaDeProyecto(ruta string) string {
	var b strings.Builder
	for _, r := range ruta {
		switch {
		case r < 128 && (unicode.IsLetter(r) || unicode.IsDigit(r)):
			b.WriteRune(r)
		case r > 0xFFFF:
			b.WriteString("--")
		default:
			b.WriteByte('-')
		}
	}
	return b.String()
}

// raizDelRepoPrincipal es la raíz del checkout principal del repo de git que contiene root, con las
// mismas piezas que ya usa la resolución de la raíz del hook (raiz.go): raizGit sube buscando `.git`
// sin correr git, y si es un worktree enlazado, principalDelWorktree sigue su `commondir` hasta el
// principal. Fuera de git devuelve root.
func raizDelRepoPrincipal(root string) string {
	top, ok := raizGit(root)
	if !ok {
		return root
	}
	if principal, ok := principalDelWorktree(top); ok {
		return principal
	}
	return top
}

// medirConImports mide un archivo y, si quedan saltos, lo que importa por @ruta. La doc lo dice
// explícito: los @imports ordenan un archivo largo pero NO le bajan el costo, porque lo importado se
// carga al arrancar junto con el que lo nombra. Sin sumarlos, un CLAUDE.md de 10 líneas que importa
// uno de 500 pasaba limpio. vistos corta los ciclos. ok=false si el archivo no existe o no se lee.
func medirConImports(ruta, home string, saltos int, vistos map[string]bool) (lineas, tamano, importados int, ok bool) {
	clave := claveDeRuta(ruta)
	if vistos[clave] {
		return 0, 0, 0, false
	}
	vistos[clave] = true
	fi, err := os.Stat(ruta)
	if err != nil || !fi.Mode().IsRegular() {
		return 0, 0, 0, false
	}
	crudo, err := os.ReadFile(ruta)
	if err != nil {
		return 0, 0, 0, false
	}
	texto := string(crudo)
	lineas, tamano = medirTexto(texto)
	if saltos <= 0 {
		return lineas, tamano, 0, true
	}
	for _, imp := range importsDe(texto) {
		destino := resolverImport(imp, filepath.Dir(ruta), home)
		if destino == "" {
			continue
		}
		l, t, n, ok := medirConImports(destino, home, saltos-1, vistos)
		if !ok {
			continue
		}
		lineas += l
		tamano += t
		importados += 1 + n
	}
	return lineas, tamano, importados, true
}

// medirTexto mide como Claude Code (ver el encabezado): recorta, cuenta los saltos de línea más uno y
// el largo en unidades UTF-16 (un carácter fuera del plano básico cuenta dos). JavaScript recorta
// también el BOM, que unicode.IsSpace no considera espacio.
func medirTexto(texto string) (lineas, tamano int) {
	t := strings.TrimFunc(texto, func(r rune) bool { return unicode.IsSpace(r) || r == 0xFEFF })
	if t == "" {
		return 0, 0
	}
	for _, r := range t {
		if r > 0xFFFF {
			tamano += 2
		} else {
			tamano++
		}
	}
	return strings.Count(t, "\n") + 1, tamano
}

// reImport reconoce un @import: una @ al principio o después de un espacio, y la ruta hasta el
// primer espacio que no esté escapado con barra invertida (la doc: «put a backslash before each
// space»). Un mail (usuario@dominio) no matchea: la @ no viene después de un espacio.
var reImport = regexp.MustCompile(`(?:^|\s)@((?:[^\s\\]|\\.)+)`)

// importsDe devuelve las rutas que el texto importa. Como Claude Code, saltea los bloques de código
// cercados y los tramos entre comillas invertidas: «`@README`» queda literal.
func importsDe(texto string) []string {
	var rutas []string
	cerca := ""
	for _, linea := range strings.Split(texto, "\n") {
		l := strings.TrimSpace(linea)
		if strings.HasPrefix(l, "```") || strings.HasPrefix(l, "~~~") {
			switch {
			case cerca == "":
				cerca = l[:3]
			case strings.HasPrefix(l, cerca):
				cerca = ""
			}
			continue
		}
		if cerca != "" {
			continue
		}
		tramos := strings.Split(linea, "`")
		for i := 0; i < len(tramos); i += 2 { // los pares quedan fuera de las comillas invertidas
			for _, m := range reImport.FindAllStringSubmatch(tramos[i], -1) {
				rutas = append(rutas, strings.ReplaceAll(m[1], `\ `, " "))
			}
		}
	}
	return rutas
}

// resolverImport lleva la ruta de un @import a una ruta del disco: `~/` es el HOME, una absoluta
// queda como está, y una relativa es relativa al archivo que importa (no al cwd), como en la doc.
func resolverImport(imp, dirDelArchivo, home string) string {
	switch {
	case strings.HasPrefix(imp, "~/") || strings.HasPrefix(imp, `~\`):
		if home == "" {
			return ""
		}
		return filepath.Join(home, imp[2:])
	case filepath.IsAbs(imp):
		return filepath.Clean(imp)
	default:
		return filepath.Join(dirDelArchivo, imp)
	}
}

// claveDeRuta normaliza una ruta para no medir dos veces el mismo archivo. En Windows no se
// distinguen mayúsculas.
func claveDeRuta(ruta string) string {
	if runtime.GOOS == "windows" {
		return strings.ToLower(filepath.Clean(ruta))
	}
	return filepath.Clean(ruta)
}

// abreviarRuta muestra una ruta con barras normales y el HOME como `~`, que es como la nombra la doc.
func abreviarRuta(ruta, home string) string {
	r, h := filepath.ToSlash(ruta), strings.TrimSuffix(filepath.ToSlash(home), "/")
	if h != "" && len(r) > len(h) && r[len(h)] == '/' && strings.EqualFold(r[:len(h)], h) {
		return "~" + r[len(h):]
	}
	return r
}

// kilos escribe un tamaño en KB de 1.000, como la doc escribe «25KB», con coma decimal.
func kilos(n int) string {
	return strings.Replace(fmt.Sprintf("%.1f KB", float64(n)/1000), ".", ",", 1)
}

// enumerar une una lista en castellano: «a», «a y b», «a, b y c».
func enumerar(xs []string) string {
	if len(xs) <= 1 {
		return strings.Join(xs, "")
	}
	return strings.Join(xs[:len(xs)-1], ", ") + " y " + xs[len(xs)-1]
}
