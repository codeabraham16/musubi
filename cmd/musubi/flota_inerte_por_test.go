package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// A131 · EL PANEL TIENE UN TEXTO PARA CADA COMPUERTA QUE PUEDE DEJAR INERTE A UNA POLÍTICA.
//
// El panel decía por qué una política estaba inerte con un texto FIJO que nombraba dos causas —la
// concesión y la allowlist— mientras la acción ya atravesaba tres (A91 le agregó el eje de
// consentimiento). Una máquina en `pide` o `prohibido` se dibujaba entonces con el motivo
// equivocado, que manda a buscar el arreglo en principals.yaml, donde no está.
//
// Desde A131 el cerebro publica la compuerta que frena en `inerte_por`, y flota.html la traduce con
// el mapa INERTE_POR. Productor y consumidor viven en dos paquetes y nada los ataba: esta guarda
// lee las constantes de `frenoDePolitica` del AST de internal/mcp/politicas.go —la fuente, no una
// lista copiada acá— y exige que el mapa tenga EXACTAMENTE esas claves. Un freno nuevo sin texto
// pone esto rojo; una clave que ya no existe, también (un texto para un estado imposible se lee
// como cobertura).
//
// ════════════════════════════════════════════════════════════════════════════════════════════
// LO QUE ESTA GUARDA YA NO MIRA, Y QUIÉN LO MIRA
//
// Hasta la revisión 2 de A131 esta prueba buscaba además, en el TEXTO de automatico(), que el mapa
// se leyera donde se arma el motivo y que el motivo se dibujara. La revisión lo midió en las dos
// direcciones y ninguna búsqueda de texto converge contra eso: el texto fijo viejo con una lectura
// muerta de `INERTE_POR[p.inerte_por]` al lado quedaba en verde, igual que `porque` mencionado y
// nunca dibujado; y el motivo en una variable intermedia o el mapa envuelto en `Object.freeze`, que
// dibujan exactamente lo mismo, la ponían roja. Eso lo contesta ahora
// TestElPanelDibujaPorQueCadaPoliticaEstaInerte, que EJECUTA el JavaScript del panel en node y mira
// lo que automatico() dibuja. Acá queda lo que se puede comprobar sin correr nada: el conjunto de
// claves contra el de constantes, en las dos direcciones.
//
// Las claves se leen del literal con o sin comillas —`'consentimiento_pide': …` es JS válido y
// equivalente—, y el literal se encuentra aunque venga envuelto (`Object.freeze({…})`): la primera
// versión castigaba la clave entre comillas, y la segunda el `Object.freeze` (falla 7 de
// sabotaje.sh).
//
// Exposición medida: 0 hoy. La única política en producción no está inerte, así que el texto no se
// dibuja; se habría dibujado mal con el primer `pide` o `prohibido` sobre musubi-server.
//
// Sabotaje: sacarle al mapa del panel el texto de `consentimiento_pide`. La otra dirección: la
// misma clave entre comillas tiene que seguir en verde.
// arnes: archivo="cmd/musubi/assets/flota.html"
// arnes: de="  consentimiento_pide: 'la máquina exige que su usuario acepte, y un barrido automático no puede esperar la respuesta',\n"
// arnes: a=""
// arnes: arreglo_de="  consentimiento_pide: 'la máquina exige"
// arnes: arreglo_a="  'consentimiento_pide': 'la máquina exige"
//
// Sabotaje: darle al mapa un texto para una compuerta que el cerebro no produce. La otra dirección:
// el mismo mapa envuelto en `Object.freeze(…)` tiene que seguir en verde.
// arnes: archivo="cmd/musubi/assets/flota.html"
// arnes: de="  mantenimiento: 'la máquina está en una ventana de mantenimiento"
// arnes: a="  freno_que_el_cerebro_no_produce: 'un estado que el cerebro ya no produce',\n  mantenimiento: 'la máquina está en una ventana de mantenimiento"
// arnes: arreglo_de="const INERTE_POR = {\n  sin_registro: 'no hay registro de principals, así que no hay a quién nombrar',\n  sin_principal: 'su principal no está en principals.yaml, o su credencial venció',\n  sin_exec: 'su principal no tiene `exec` sobre esta máquina, o la máquina no lo admite',\n  allowlist: 'el comando no está en la allowlist de su principal',\n  consentimiento_prohibido: 'la máquina tiene el consentimiento en `prohibido` (o en `pide` sin forma de preguntar)',\n  consentimiento_pide: 'la máquina exige que su usuario acepte, y un barrido automático no puede esperar la respuesta',\n  mantenimiento: 'la máquina está en una ventana de mantenimiento, y ninguna política actúa sobre ella hasta que la ventana cierre o se cancele',\n};\n"
// arnes: arreglo_a="const INERTE_POR = Object.freeze({\n  sin_registro: 'no hay registro de principals, así que no hay a quién nombrar',\n  sin_principal: 'su principal no está en principals.yaml, o su credencial venció',\n  sin_exec: 'su principal no tiene `exec` sobre esta máquina, o la máquina no lo admite',\n  allowlist: 'el comando no está en la allowlist de su principal',\n  consentimiento_prohibido: 'la máquina tiene el consentimiento en `prohibido` (o en `pide` sin forma de preguntar)',\n  consentimiento_pide: 'la máquina exige que su usuario acepte, y un barrido automático no puede esperar la respuesta',\n  mantenimiento: 'la máquina está en una ventana de mantenimiento, y ninguna política actúa sobre ella hasta que la ventana cierre o se cancele',\n});\n"
//
// Sabotaje: que vistasJS deje de saltear la barra de escape adentro de un string. `'it\'s // q'`
// termina entonces en la comilla escapada, y lo que sigue se lee como comentario: lo ve el control
// del instrumento, que hasta la revisión 2 no tenía ningún escape y con este sabotaje quedaba verde.
// arnes: archivo="cmd/musubi/flota_inerte_por_test.go"
// arnes: de="\t\t\t\tif src[j] == '\\\\' {\n"
// arnes: a="\t\t\t\tif false && src[j] == '\\\\' {\n"
func TestElPanelTieneTextoParaCadaFrenoDePolitica(t *testing.T) {
	// ── EL INSTRUMENTO, ANTES DE CREERLE ───────────────────────────────────────────────────────
	// Las claves se leen a través de vistasJS. Si el lexer dejara de separar un comentario de un
	// string —o de saltear una comilla escapada—, una clave comentada se contaría como texto, o
	// una entrada real se perdería, y ninguna comprobación de abajo lo diría.
	{
		muestra := "x = 'a // b' + `c ${y + 'd'} e`; // INERTE_POR[z]\n/* w */ v = \"f\";\nu = 'it\\'s // q'; // ZZ"
		sinCom, cod := vistasJS(muestra)
		if len(sinCom) != len(muestra) || len(cod) != len(muestra) {
			t.Fatalf("vistasJS cambió el largo del texto: las dos vistas tienen que alinearse byte a byte con el original")
		}
		if strings.Contains(sinCom, "INERTE_POR") || strings.Contains(sinCom, "w") || strings.Contains(sinCom, "ZZ") {
			t.Fatalf("vistasJS no blanqueó los comentarios: %q", sinCom)
		}
		if !strings.Contains(sinCom, "'a // b'") || !strings.Contains(sinCom, `"f"`) || !strings.Contains(sinCom, `'it\'s // q'`) {
			t.Fatalf("vistasJS se comió un string como si fuera un comentario: %q", sinCom)
		}
		if strings.Contains(cod, "a // b") || strings.Contains(cod, "c ") || strings.Contains(cod, "// q") || !strings.Contains(cod, "${y + '") {
			t.Fatalf("la vista de código tiene que blanquear el CONTENIDO de los literales y conservar las expresiones ${…}: %q", cod)
		}
	}

	// ── EL PRODUCTOR: las constantes del cerebro ─────────────────────────────────────────────
	frenos := frenosDelCerebro(t)

	// ── EL CONSUMIDOR: las claves del mapa, en el script del asset EMBEBIDO que se sirve ───────
	guion := scriptDelPanel(t)
	decl := regexp.MustCompile(`(?m)^[ \t]*(?:const|let|var)\s+INERTE_POR\s*=`).FindStringIndex(guion)
	if decl == nil {
		t.Fatal("flota.html no tiene el mapa INERTE_POR: el panel volvió a un texto fijo, que es el defecto de A131")
	}
	// Se lexea DESDE la declaración: lo que venga después del mapa no puede correr nada de lo que
	// hay adentro, y vistasJS no reconoce literales de regex (esc() tiene uno, más abajo).
	mapaSin, mapaCod := vistasJS(guion[decl[0]:])
	igual := decl[1] - decl[0]
	// El literal es la primera llave del inicializador, venga pelado o envuelto: `= {…}` y
	// `= Object.freeze({…})` declaran el mismo mapa.
	fin := finDeSentenciaJS(mapaCod, igual)
	abre := strings.IndexByte(mapaCod[igual:fin], '{')
	if abre < 0 {
		t.Fatal("INERTE_POR no se inicializa con un literal de objeto en flota.html: esta guarda no sabe leer sus " +
			"claves, y adivinarlas sería inventar cobertura")
	}
	abre += igual
	cierra := cierreJS(mapaCod, abre)
	if cierra < 0 {
		t.Fatal("no encontré la llave que cierra el mapa INERTE_POR en flota.html")
	}
	claves, err := clavesDeObjetoJS(mapaSin[abre+1:cierra], mapaCod[abre+1:cierra])
	if err != nil {
		t.Fatalf("el mapa INERTE_POR tiene una entrada que esta guarda no sabe leer, y adivinar su clave sería inventar cobertura: %v", err)
	}
	if len(claves) == 0 {
		t.Fatal("el mapa INERTE_POR no tiene ninguna clave legible: la guarda no midió nada")
	}

	var sinTexto, sobran []string
	for fr := range frenos {
		if !claves[fr] {
			sinTexto = append(sinTexto, fr)
		}
	}
	for c := range claves {
		if !frenos[c] {
			sobran = append(sobran, c)
		}
	}
	sort.Strings(sinTexto)
	sort.Strings(sobran)
	if len(sinTexto) > 0 {
		t.Errorf("el panel no tiene texto para %s: una política frenada por esa compuerta se dibuja con el "+
			"código crudo en vez de decirle a quien mira dónde arreglarla. Agregala a INERTE_POR en flota.html",
			strings.Join(sinTexto, ", "))
	}
	if len(sobran) > 0 {
		t.Errorf("INERTE_POR tiene texto para %s, que no es ninguna constante de frenoDePolitica: explica un "+
			"estado que el cerebro ya no produce", strings.Join(sobran, ", "))
	}
}

// vistasJS lexea `src` como JavaScript y devuelve dos vistas del MISMO largo, alineadas byte a byte
// con el original (así un índice encontrado en una sirve en la otra):
//
//   - sinComentarios: los comentarios `//` y `/* */` en blanco; strings y plantillas intactos.
//   - codigo: además, el CONTENIDO de los literales en blanco. Quedan las comillas, los backticks y
//     las expresiones `${…}` de las plantillas, que son código.
//
// Los saltos de línea se conservan siempre. NO reconoce literales de regex: el mapa INERTE_POR no
// tiene ninguno y se lexea desde su declaración, así que los que vengan después (el de esc(), por
// ejemplo) no lo alcanzan. El control del principio de la prueba es lo que avisa si este lexer deja
// de separar un comentario de un string, o de saltear una comilla escapada.
func vistasJS(src string) (sinComentarios, codigo string) {
	sin := []byte(src)
	cod := []byte(src)
	blanco := func(b []byte, desde, hasta int) {
		for k := desde; k < hasta && k < len(b); k++ {
			if b[k] != '\n' {
				b[k] = ' '
			}
		}
	}
	// Una pila de marcos: código (con su cuenta de llaves) o plantilla. Una expresión `${` abre un
	// marco de código encima de la plantilla, y su `}` sin pareja lo cierra.
	type marco struct {
		plantilla bool
		llaves    int
	}
	pila := []marco{{}}
	for i := 0; i < len(src); {
		tope := &pila[len(pila)-1]
		c := src[i]
		if tope.plantilla {
			switch {
			case c == '\\':
				blanco(cod, i, i+2)
				i += 2
			case c == '`':
				pila = pila[:len(pila)-1]
				i++
			case c == '$' && i+1 < len(src) && src[i+1] == '{':
				pila = append(pila, marco{})
				i += 2
			default:
				blanco(cod, i, i+1)
				i++
			}
			continue
		}
		switch {
		case c == '/' && i+1 < len(src) && src[i+1] == '/':
			fin := strings.IndexByte(src[i:], '\n')
			if fin < 0 {
				fin = len(src) - i
			}
			blanco(sin, i, i+fin)
			blanco(cod, i, i+fin)
			i += fin
		case c == '/' && i+1 < len(src) && src[i+1] == '*':
			hasta := len(src)
			if fin := strings.Index(src[i+2:], "*/"); fin >= 0 {
				hasta = i + 2 + fin + 2
			}
			blanco(sin, i, hasta)
			blanco(cod, i, hasta)
			i = hasta
		case c == '\'' || c == '"':
			j := i + 1
			for j < len(src) && src[j] != c && src[j] != '\n' {
				if src[j] == '\\' {
					j++
				}
				j++
			}
			blanco(cod, i+1, j)
			i = j + 1
		case c == '`':
			pila = append(pila, marco{plantilla: true})
			i++
		case c == '{':
			tope.llaves++
			i++
		case c == '}':
			if tope.llaves == 0 && len(pila) > 1 {
				pila = pila[:len(pila)-1] // cierra la expresión ${…}: se vuelve a la plantilla
			} else {
				tope.llaves--
			}
			i++
		default:
			i++
		}
	}
	return string(sin), string(cod)
}

// cierreJS devuelve el índice del delimitador que cierra al que está en codigo[abre], o -1. Cuenta
// ([{ contra )]} sobre la vista de CÓDIGO, donde el contenido de los literales ya está en blanco:
// un corchete adentro de un string no lo corre.
func cierreJS(codigo string, abre int) int {
	if abre < 0 || abre >= len(codigo) || !strings.ContainsRune("([{", rune(codigo[abre])) {
		return -1
	}
	prof := 0
	for i := abre; i < len(codigo); i++ {
		switch codigo[i] {
		case '(', '[', '{':
			prof++
		case ')', ']', '}':
			prof--
			if prof == 0 {
				return i
			}
		}
	}
	return -1
}

// finDeSentenciaJS devuelve dónde termina la sentencia que arranca en `desde`: el primer `;` a
// profundidad cero, o el delimitador que cierra el bloque que la contiene. Sin ninguno de los dos,
// el final del texto — más ancho, nunca más angosto: una sentencia sin `;` alarga la zona donde
// se busca, no la achica hasta dejar afuera lo que sí está.
func finDeSentenciaJS(codigo string, desde int) int {
	prof := 0
	for i := desde; i < len(codigo); i++ {
		switch codigo[i] {
		case '(', '[', '{':
			prof++
		case ')', ']', '}':
			prof--
			if prof < 0 {
				return i
			}
		case ';':
			if prof == 0 {
				return i
			}
		}
	}
	return len(codigo)
}

// clavesDeObjetoJS lee las claves de un literal de objeto (el texto ENTRE sus llaves, en las dos
// vistas de vistasJS). Cada entrada se separa por las comas a profundidad cero de la vista de
// código, y su clave es lo que está antes de su primer `:` —identificador pelado o entre comillas,
// que para JS son lo mismo—. Una entrada que no tenga esa forma (una clave calculada, un `...`)
// es un error y no una clave de menos: contarla como ausente pediría un texto que ya existe.
func clavesDeObjetoJS(sinComentarios, codigo string) (map[string]bool, error) {
	claves := map[string]bool{}
	nombre := regexp.MustCompile(`^(?:([A-Za-z_$][A-Za-z0-9_$]*)|'([^'\\]*)'|"([^"\\]*)")$`)
	desde, prof := 0, 0
	entrada := func(hasta int) error {
		cod := codigo[desde:hasta]
		if strings.TrimSpace(cod) == "" {
			return nil // la coma final después de la última entrada
		}
		dos := strings.IndexByte(cod, ':')
		if dos < 0 {
			return fmt.Errorf("la entrada %q no tiene `clave:`", strings.TrimSpace(sinComentarios[desde:hasta]))
		}
		crudo := strings.TrimSpace(sinComentarios[desde : desde+dos])
		m := nombre.FindStringSubmatch(crudo)
		if m == nil {
			return fmt.Errorf("la clave %q no es un nombre ni un string", crudo)
		}
		claves[m[1]+m[2]+m[3]] = true
		return nil
	}
	for i := 0; i < len(codigo); i++ {
		switch codigo[i] {
		case '(', '[', '{':
			prof++
		case ')', ']', '}':
			prof--
		case ',':
			if prof == 0 {
				if err := entrada(i); err != nil {
					return nil, err
				}
				desde = i + 1
			}
		}
	}
	if err := entrada(len(codigo)); err != nil {
		return nil, err
	}
	return claves, nil
}

// frenosDelCerebro devuelve los valores de las constantes de tipo `frenoDePolitica`, leídos del
// AST de internal/mcp/politicas.go —la fuente, no una lista copiada acá—, sin el vacío: `sinFreno`
// es «ninguna compuerta frena» y nunca viaja como `inerte_por`. Lo usan las dos guardas del panel:
// TestElPanelTieneTextoParaCadaFrenoDePolitica, para el conjunto de claves, y
// TestElPanelDibujaPorQueCadaPoliticaEstaInerte, para recorrer cada freno aunque el mapa no lo
// conozca.
func frenosDelCerebro(t *testing.T) map[string]bool {
	t.Helper()
	fuente := filepath.Join("..", "..", "internal", "mcp", "politicas.go")
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, fuente, nil, 0)
	if err != nil {
		t.Fatalf("no se pudo parsear %s: %v", fuente, err)
	}
	frenos := map[string]bool{}
	for _, decl := range f.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok || gd.Tok != token.CONST {
			continue
		}
		for _, spec := range gd.Specs {
			vs, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			if tipo, ok := vs.Type.(*ast.Ident); !ok || tipo.Name != "frenoDePolitica" {
				continue
			}
			for _, v := range vs.Values {
				lit, ok := v.(*ast.BasicLit)
				if !ok || lit.Kind != token.STRING {
					t.Fatalf("%s: una constante de frenoDePolitica no es un literal de cadena y esta guarda no la puede leer", fset.Position(v.Pos()))
				}
				valor, err := strconv.Unquote(lit.Value)
				if err != nil {
					t.Fatalf("%s: %v", fset.Position(v.Pos()), err)
				}
				if valor != "" {
					frenos[valor] = true
				}
			}
		}
	}
	// PISO: cero frenos no es «no hay nada que dibujar», es que el tipo se renombró o se mudó.
	if len(frenos) < 5 {
		t.Fatalf("encontré %d constantes de frenoDePolitica en %s y son al menos cinco: la guarda dejó de ver la fuente", len(frenos), fuente)
	}
	return frenos
}

// scriptDelPanel devuelve el JavaScript de flota.html TAL COMO SE SIRVE: el contenido de sus bloques
// <script>, en orden, sacado del asset EMBEBIDO. Las dos guardas del panel lo toman de acá, así
// que miran los mismos bytes —una leyéndolos y la otra ejecutándolos—.
func scriptDelPanel(t *testing.T) string {
	t.Helper()
	p := string(assetsFS(t, "assets/flota.html"))
	var guion strings.Builder
	for resto := p; ; {
		i := strings.Index(resto, "<script>")
		if i < 0 {
			break
		}
		resto = resto[i+len("<script>"):]
		j := strings.Index(resto, "</script>")
		if j < 0 {
			t.Fatal("flota.html abre un <script> que no cierra")
		}
		guion.WriteString(resto[:j])
		guion.WriteString("\n")
		resto = resto[j+len("</script>"):]
	}
	if guion.Len() == 0 {
		t.Fatal("flota.html no tiene ningún <script>: el panel no tiene código que estas guardas puedan mirar")
	}
	return guion.String()
}
