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
// LA PRIMERA VERSIÓN DE ESTA GUARDA TENÍA LOS DOS DEFECTOS DE SIEMPRE, UNO PARA CADA LADO
//
//   - LA MIRABA UN COMENTARIO. El uso del mapa se comprobaba con un `strings.Contains` de
//     `INERTE_POR[p.inerte_por]` sobre el archivo ENTERO. La revisión de A131 devolvió automatico()
//     al texto fijo de dos causas —el defecto exacto de LD1/P3-L1— dejando ese literal en un
//     comentario al final de la línea: sabotaje.sh dijo «EL SABOTAJE NO LA PONE EN ROJO» y
//     `go test ./cmd/musubi` entero dio ok. Ahora se busca en el CÓDIGO (sin comentarios ni
//     contenido de literales) del cuerpo de automatico(), y adentro de la expresión que arma
//     `porque`, que es el texto que se dibuja; y se exige además que `porque` se dibuje.
//   - CASTIGABA EL ARREGLO (falla 7 de sabotaje.sh). Las claves se leían con `^\s*([a-z_]+):`,
//     así que `'consentimiento_pide': …` —JS válido y equivalente— la ponía roja; y el uso se
//     buscaba escrito con punto, así que `INERTE_POR[p['inerte_por']]` también. Ahora las claves
//     se leen del objeto con o sin comillas, y el uso es «un índice de INERTE_POR que nombra
//     `inerte_por`», se escriba como se escriba el acceso.
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
// Sabotaje: volver automatico() al texto fijo de dos causas, dejando el literal del uso en un
// comentario. La otra dirección: el acceso escrito con corchetes y comillas sigue en verde.
// arnes: archivo="cmd/musubi/assets/flota.html"
// arnes: de="      ` — INERTE: ${INERTE_POR[p.inerte_por] || 'la frena la compuerta «' + (p.inerte_por || 'sin motivo informado') + '»'}. No va a actuar.`;\n"
// arnes: a="      ' — INERTE: su principal no tiene `exec` sobre esta máquina, o el comando no está en su allowlist. No va a actuar.'; // INERTE_POR[p.inerte_por]\n"
// arnes: arreglo_de="INERTE_POR[p.inerte_por] ||"
// arnes: arreglo_a="INERTE_POR[p['inerte_por']] ||"
//
// Sabotaje: armar `porque` con el mapa y no dibujarlo. La otra dirección: dibujarlo escapado
// aparte tiene que seguir en verde.
// arnes: archivo="cmd/musubi/assets/flota.html"
// arnes: de="title=\"${esc(t + porque)}\""
// arnes: a="title=\"${esc(t)}\""
// arnes: arreglo_de="title=\"${esc(t + porque)}\""
// arnes: arreglo_a="title=\"${esc(t) + esc(porque)}\""
func TestElPanelTieneTextoParaCadaFrenoDePolitica(t *testing.T) {
	// ── EL INSTRUMENTO, ANTES DE CREERLE ───────────────────────────────────────────────────────
	// Todo lo de abajo mira el panel a través de vistasJS. Si el lexer dejara de separar un
	// comentario de un string, las comprobaciones volverían a contentarse con un texto que no
	// decide nada —o a acusar uno que sí—, y ninguna lo diría.
	{
		muestra := "x = 'a // b' + `c ${y + 'd'} e`; // INERTE_POR[z]\n/* w */ v = \"f\";"
		sinCom, cod := vistasJS(muestra)
		if len(sinCom) != len(muestra) || len(cod) != len(muestra) {
			t.Fatalf("vistasJS cambió el largo del texto: las dos vistas tienen que alinearse byte a byte con el original")
		}
		if strings.Contains(sinCom, "INERTE_POR") || strings.Contains(sinCom, "w") {
			t.Fatalf("vistasJS no blanqueó los comentarios: %q", sinCom)
		}
		if !strings.Contains(sinCom, "'a // b'") || !strings.Contains(sinCom, `"f"`) {
			t.Fatalf("vistasJS se comió un string como si fuera un comentario: %q", sinCom)
		}
		if strings.Contains(cod, "a // b") || strings.Contains(cod, "c ") || !strings.Contains(cod, "${y + '") {
			t.Fatalf("la vista de código tiene que blanquear el CONTENIDO de los literales y conservar las expresiones ${…}: %q", cod)
		}
	}

	// ── EL PRODUCTOR: las constantes del cerebro ─────────────────────────────────────────────
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
				// El vacío es «ninguna compuerta frena»: nunca viaja como `inerte_por`.
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

	// ── EL CONSUMIDOR: las claves del mapa, en el asset EMBEBIDO que se sirve ──────────────────
	p := string(assetsFS(t, "assets/flota.html"))
	const cabecera = "const INERTE_POR = "
	ini := strings.Index(p, cabecera+"{")
	if ini < 0 {
		t.Fatal("flota.html no tiene el mapa INERTE_POR: el panel volvió a un texto fijo, que es el defecto de A131")
	}
	// Se lexea DESDE la llave del mapa: lo que venga después de su cierre no puede correr nada de
	// lo que hay adentro.
	mapaSin, mapaCod := vistasJS(p[ini+len(cabecera):])
	cierra := cierreJS(mapaCod, 0)
	if cierra < 0 {
		t.Fatal("no encontré la llave que cierra el mapa INERTE_POR en flota.html")
	}
	claves, err := clavesDeObjetoJS(mapaSin[1:cierra], mapaCod[1:cierra])
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

	// ── Y EL MAPA SE USA DONDE SE ARMA EL MOTIVO QUE SE DIBUJA, no en cualquier texto ─────────
	fn := strings.Index(p, "function automatico(")
	if fn < 0 {
		t.Fatal("flota.html no tiene automatico(): la columna de lo automático dejó de existir o cambió de nombre, y esta guarda no está mirando nada")
	}
	fnSin, fnCod := vistasJS(p[fn:])
	llave := strings.IndexByte(fnCod, '{')
	fin := -1
	if llave >= 0 {
		fin = cierreJS(fnCod, llave)
	}
	if fin < 0 {
		t.Fatal("no encontré el cuerpo de automatico() en flota.html")
	}
	cuerpoSin, cuerpoCod := fnSin[llave:fin+1], fnCod[llave:fin+1]
	decl := regexp.MustCompile(`\b(?:const|let|var)\s+porque\s*=`).FindStringIndex(cuerpoCod)
	if decl == nil {
		t.Fatal("automatico() no arma `porque`, el texto de por qué una política está inerte: sin él el panel " +
			"no puede decir cuál compuerta la frena. (Si la variable cambió de nombre, esta guarda tiene que seguirla.)")
	}
	finDecl := finDeSentenciaJS(cuerpoCod, decl[1])
	traduce := false
	for _, m := range regexp.MustCompile(`\bINERTE_POR\s*\[`).FindAllStringIndex(cuerpoCod[decl[1]:finDecl], -1) {
		abre := decl[1] + m[1] - 1
		cierraIdx := cierreJS(cuerpoCod, abre)
		if cierraIdx < 0 || cierraIdx > finDecl {
			continue
		}
		// El ÍNDICE se lee de la vista con los literales intactos: `p['inerte_por']` nombra el campo
		// adentro de un string, y es tan válido como `p.inerte_por`.
		if regexp.MustCompile(`\binerte_por\b`).MatchString(cuerpoSin[abre+1 : cierraIdx]) {
			traduce = true
		}
	}
	if !traduce {
		t.Errorf("la expresión que arma `porque` en automatico() no indexa INERTE_POR con `inerte_por`: el mapa " +
			"existe y el panel no lo lee, así que una política inerte se explica con un texto que no depende de " +
			"la compuerta que la frena — el defecto de A131. (Un comentario o un string que lo nombre no cuenta.)")
	}
	if !regexp.MustCompile(`\bporque\b`).MatchString(cuerpoCod[finDecl:]) {
		t.Errorf("automatico() arma `porque` y nunca lo usa: el motivo traducido no llega a lo que se dibuja, y " +
			"una política inerte se ve sin decir por qué")
	}
}

// vistasJS lexea `src` como JavaScript y devuelve dos vistas del MISMO largo, alineadas byte a byte
// con el original (así un índice encontrado en una sirve en la otra):
//
//   - sinComentarios: los comentarios `//` y `/* */` en blanco; strings y plantillas intactos.
//   - codigo: además, el CONTENIDO de los literales en blanco. Quedan las comillas, los backticks y
//     las expresiones `${…}` de las plantillas, que son código.
//
// Los saltos de línea se conservan siempre. NO reconoce literales de regex: automatico() y el mapa
// INERTE_POR no tienen ninguno, y el control del principio de la prueba es lo que avisa si este
// lexer deja de separar un comentario de un string.
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
