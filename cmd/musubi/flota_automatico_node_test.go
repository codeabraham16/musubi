package main

import (
	"bytes"
	"context"
	"encoding/json"
	"html"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"

	"musubi/internal/guiones"
)

// claveQueElPanelNoConoce es un `inerte_por` que ninguna versión del cerebro produce. El panel
// tiene que dibujarlo CRUDO: un motivo desconocido se muestra, nunca se reemplaza por una causa
// inventada.
const claveQueElPanelNoConoce = "freno_que_este_panel_no_conoce"

// A131 · revisión 2 · EL PANEL DIBUJA POR QUÉ CADA POLÍTICA ESTÁ INERTE, Y ESO SE MIDE CORRIÉNDOLO.
//
// El defecto de LD1/P3-L1 tenía dos mitades y ésta es la del panel: automatico() explicaba toda
// política inerte con un texto FIJO de dos causas, así que una máquina en `pide` o `prohibido`
// mandaba a buscar el arreglo en principals.yaml, donde no está. Desde A131 el cerebro publica
// `inerte_por` y el panel lo traduce con el mapa INERTE_POR.
//
// ════════════════════════════════════════════════════════════════════════════════════════════
// POR QUÉ SE EJECUTA Y NO SE LEE
//
// La guarda anterior buscaba en el TEXTO de automatico() que la expresión de `porque` indexara
// INERTE_POR con `inerte_por`, y que `porque` apareciera después. La revisión 2 de A131 la midió
// en las dos direcciones y falló en las dos:
//
//   - VERDE CON EL DEFECTO: el texto fijo viejo con una lectura muerta de
//     `INERTE_POR[p.inerte_por]` en la misma expresión (le suma un string vacío), y `porque`
//     mencionado en una sentencia que no dibuja nada. Ejecutado en node, el primero dibujaba el
//     texto viejo en los nueve casos, y el paquete entero seguía en `ok`.
//   - ROJA CON EL ARREGLO (falla 7 de sabotaje.sh): el motivo en una variable intermedia, o el
//     mapa envuelto en `Object.freeze`, que dibujan exactamente lo mismo.
//
// Una guarda que pregunta por una MENCIÓN no converge: las formas de mencionar sin decidir —y de
// decidir sin la mención esperada— no se acaban. Lo que decide es lo que se DIBUJA, y eso se
// contesta ejecutando. Esta prueba toma el <script> del asset EMBEBIDO (los mismos bytes que se
// sirven), lo evalúa en node con lo mínimo del navegador, y le pide a automatico() la columna de
// una política inerte por cada clave de INERTE_POR, por cada constante de `frenoDePolitica` del
// cerebro y por una clave que nadie produce. Exige:
//
//  1. que lo DIBUJADO —el texto visible y los `title`, con las entidades de HTML resueltas—
//     contenga el texto que INERTE_POR tiene para esa clave, o la clave cruda si el mapa no la
//     conoce;
//  2. que dos claves distintas no se dibujen igual: dos compuertas con la misma explicación mandan
//     a arreglar las dos al mismo lugar, que es el defecto de A131 en chico;
//  3. que una política SANA no se dibuje INERTE, ni con la palabra ni con ningún texto del mapa.
//
// Que el mapa tenga una clave por cada constante del cerebro, y ninguna de más, lo sigue exigiendo
// —sin node— TestElPanelTieneTextoParaCadaFrenoDePolitica.
//
// SIN NODE, EN CI ES UN FALLO Y AFUERA UN SALTEO QUE DICE POR QUÉ. Las imágenes de los runners de
// GitHub traen node preinstalado (y el job `panel` de ci.yml fija la 24), así que una ausencia en
// CI es un runner roto, y un verde ahí sería «no medí» contestando lo mismo que «medí y está bien».
// En una máquina de desarrollo sin node, en cambio, frenar `go test` entero por esto enseñaría a
// apagarla. El proceso se lanza con guiones.HerramientaCtx, el único camino por el que una prueba
// de este repo arranca un proceso.
//
// Exposición medida: 0 hoy. La única política en producción no está inerte, así que el texto no
// se dibuja; con el primer `pide` o `prohibido` sobre musubi-server se habría dibujado el viejo.
//
// Sabotaje: volver automatico() al texto fijo de dos causas —el defecto de LD1/P3-L1 en el panel—
// dejando una lectura MUERTA de `INERTE_POR[p.inerte_por]` en la misma expresión. La otra
// dirección: sacar el motivo a una variable intermedia dibuja lo mismo y tiene que seguir en verde.
// arnes: archivo="cmd/musubi/assets/flota.html"
// arnes: de="      ` — INERTE: ${INERTE_POR[p.inerte_por] || 'la frena la compuerta «' + (p.inerte_por || 'sin motivo informado') + '»'}. No va a actuar.`;\n"
// arnes: a="      ' — INERTE: su principal no tiene `exec` sobre esta máquina, o el comando no está en su allowlist. No va a actuar.' + (INERTE_POR[p.inerte_por] ? \x27\x27 : \x27\x27);\n"
// arnes: arreglo_de="    const porque = p.puede_actuar ? \x27\x27 :\n      ` — INERTE: ${INERTE_POR[p.inerte_por] || 'la frena la compuerta «' + (p.inerte_por || 'sin motivo informado') + '»'}. No va a actuar.`;\n"
// arnes: arreglo_a="    const motivo = INERTE_POR[p.inerte_por] || 'la frena la compuerta «' + (p.inerte_por || 'sin motivo informado') + '»';\n    const porque = p.puede_actuar ? \x27\x27 : ` — INERTE: ${motivo}. No va a actuar.`;\n"
//
// Sabotaje: armar `porque` y no dibujarlo, MENCIONÁNDOLO en otra sentencia (`const titulo = t +
// porque;`) para que una búsqueda de texto lo diera por usado. La otra dirección: el mapa envuelto
// en `Object.freeze(…)` dibuja lo mismo y tiene que seguir en verde.
// arnes: archivo="cmd/musubi/assets/flota.html"
// arnes: de="    return `<span class=\"${clase}\" title=\"${esc(t + porque)}\">⚙ ${esc(p.nombre)}${marca}</span>`;\n"
// arnes: a="    const titulo = t + porque;\n    return `<span class=\"${clase}\" title=\"${esc(t)}\">⚙ ${esc(p.nombre)}${marca}</span>`;\n"
// arnes: arreglo_de="const INERTE_POR = {\n  sin_registro: 'no hay registro de principals, así que no hay a quién nombrar',\n  sin_principal: 'su principal no está en principals.yaml, o su credencial venció',\n  sin_exec: 'su principal no tiene `exec` sobre esta máquina, o la máquina no lo admite',\n  allowlist: 'el comando no está en la allowlist de su principal',\n  consentimiento_prohibido: 'la máquina tiene el consentimiento en `prohibido` (o en `pide` sin forma de preguntar)',\n  consentimiento_pide: 'la máquina exige que su usuario acepte, y un barrido automático no puede esperar la respuesta',\n  mantenimiento: 'la máquina está en una ventana de mantenimiento, y ninguna política actúa sobre ella hasta que la ventana cierre o se cancele',\n};\n"
// arnes: arreglo_a="const INERTE_POR = Object.freeze({\n  sin_registro: 'no hay registro de principals, así que no hay a quién nombrar',\n  sin_principal: 'su principal no está en principals.yaml, o su credencial venció',\n  sin_exec: 'su principal no tiene `exec` sobre esta máquina, o la máquina no lo admite',\n  allowlist: 'el comando no está en la allowlist de su principal',\n  consentimiento_prohibido: 'la máquina tiene el consentimiento en `prohibido` (o en `pide` sin forma de preguntar)',\n  consentimiento_pide: 'la máquina exige que su usuario acepte, y un barrido automático no puede esperar la respuesta',\n  mantenimiento: 'la máquina está en una ventana de mantenimiento, y ninguna política actúa sobre ella hasta que la ventana cierre o se cancele',\n});\n"
//
// Sabotaje: explicar dos compuertas distintas con el MISMO texto —`sin_principal` con el de
// `sin_registro`—: cada una sigue encontrando «su» texto, y sólo la comparación entre claves lo
// ve. La otra dirección: la misma clave entre comillas tiene que seguir en verde.
// arnes: archivo="cmd/musubi/assets/flota.html"
// arnes: de="  sin_principal: 'su principal no está en principals.yaml, o su credencial venció',\n"
// arnes: a="  sin_principal: 'no hay registro de principals, así que no hay a quién nombrar',\n"
// arnes: arreglo_de="  sin_principal: 'su principal"
// arnes: arreglo_a="  'sin_principal': 'su principal"
//
// Sabotaje: armar el motivo aunque la política PUEDA actuar (el ternario deja de mirar
// `puede_actuar`): la política sana se dibuja «INERTE: la frena la compuerta «sin motivo
// informado»», que no es ningún texto del mapa, y las inertes siguen diciendo lo suyo. Lo ve sólo
// la comparación de la sana.
// arnes: archivo="cmd/musubi/assets/flota.html"
// arnes: de="    const porque = p.puede_actuar ? \x27\x27 :\n"
// arnes: a="    const porque = false ? \x27\x27 :\n"
func TestElPanelDibujaPorQueCadaPoliticaEstaInerte(t *testing.T) {
	nodeParaElPanel(t)

	frenos := make([]string, 0, 8)
	for f := range frenosDelCerebro(t) {
		frenos = append(frenos, f)
	}
	sort.Strings(frenos)
	r := automaticoEnNode(t, scriptDelPanel(t), frenos)

	// PISOS: sin textos, o sin la fila de algún freno, el recorrido de abajo mediría menos de lo
	// que dice —y un mapa vacío lo dejaría en verde sin haber comparado nada—.
	if len(r.Textos) == 0 {
		t.Fatal("el mapa INERTE_POR evaluado en node no tiene ninguna clave: esta prueba no compararía nada")
	}
	for _, k := range append(append([]string{}, frenos...), claveQueElPanelNoConoce) {
		if _, ok := r.Dibujos[k]; !ok {
			t.Fatalf("node no devolvió lo que automatico() dibuja con `inerte_por: %q`: el arnés no recorrió esa clave", k)
		}
	}

	claves := make([]string, 0, len(r.Dibujos))
	for k := range r.Dibujos {
		claves = append(claves, k)
	}
	sort.Strings(claves)

	// 1. CADA CLAVE DIBUJA SU TEXTO, o la clave cruda si el mapa no la conoce.
	porDibujo := map[string][]string{}
	for _, k := range claves {
		dibujo := dibujadoDe(r.Dibujos[k])
		porDibujo[dibujo] = append(porDibujo[dibujo], k)
		if texto, conocida := r.Textos[k]; conocida {
			if strings.TrimSpace(texto) == "" {
				t.Errorf("INERTE_POR tiene un texto VACÍO para %q: no explica nada, y como en JS un string vacío es "+
					"falso, el panel cae al motivo crudo como si no conociera la compuerta", k)
			} else if !strings.Contains(dibujo, texto) {
				t.Errorf("con `inerte_por: %q` el panel dibuja %q, que no contiene el texto que INERTE_POR tiene "+
					"para esa compuerta (%q): la política inerte se explica con algo que no depende de la compuerta "+
					"que la frena, que es el defecto de A131", k, dibujo, texto)
			}
		} else if !strings.Contains(dibujo, k) {
			t.Errorf("con `inerte_por: %q`, que INERTE_POR no conoce, el panel dibuja %q sin el motivo crudo: "+
				"lo desconocido se muestra, nunca se reemplaza por una causa inventada", k, dibujo)
		}
	}

	// 2. DOS CLAVES DISTINTAS NO SE DIBUJAN IGUAL.
	dibujos := make([]string, 0, len(porDibujo))
	for d := range porDibujo {
		dibujos = append(dibujos, d)
	}
	sort.Strings(dibujos)
	for _, d := range dibujos {
		if ks := porDibujo[d]; len(ks) > 1 {
			t.Errorf("las compuertas %s se dibujan IGUAL (%q): el panel manda a arreglarlas al mismo lugar, y "+
				"a lo sumo una de las dos está ahí", strings.Join(ks, ", "), d)
		}
	}

	// 3. UNA POLÍTICA SANA NO SE DIBUJA INERTE, ni con la palabra ni con ningún texto del mapa: un
	// `porque` armado sin mirar `puede_actuar` dibuja «INERTE: la frena la compuerta «sin motivo
	// informado»» sobre una política que actúa, y eso no es ningún texto de INERTE_POR.
	sana := dibujadoDe(r.Sana)
	if strings.Contains(strings.ToLower(sana), "inerte") {
		t.Errorf("una política con `puede_actuar: true` se dibuja como INERTE (%q): el panel marca inerte a una "+
			"política que actúa", sana)
	}
	textos := make([]string, 0, len(r.Textos))
	for k := range r.Textos {
		textos = append(textos, k)
	}
	sort.Strings(textos)
	for _, k := range textos {
		if strings.Contains(sana, r.Textos[k]) {
			t.Errorf("una política con `puede_actuar: true` se dibuja con el texto de %q (%q): el panel marca "+
				"inerte a una política que actúa", k, sana)
		}
	}
}

// nodeParaElPanel exige node para ejecutar el JavaScript del panel: en CI su ausencia es un FALLO,
// y afuera un salteo con el motivo (ver el doc de TestElPanelDibujaPorQueCadaPoliticaEstaInerte).
func nodeParaElPanel(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("node"); err == nil {
		return
	}
	if enCI() {
		t.Fatalf("no hay `node` en el PATH y esto corre en CI (CI=%q): la guarda del panel no puede ejecutar "+
			"automatico(), y un salteo acá sería «no medí» con la cara de «medí y está bien». Las imágenes de "+
			"los runners de GitHub lo traen; si éste no, el runner está roto", os.Getenv("CI"))
	}
	t.Skip("SALTEADA: no hay `node` en el PATH. Esta prueba EJECUTA el JavaScript de flota.html para ver qué " +
		"dibuja automatico() con cada `inerte_por`, y sin node no puede. En CI (con la variable CI puesta) " +
		"esto es un FALLO y no un salteo. Instalá node para medirla acá.")
}

// enCI contesta si la prueba corre en integración continua, por la variable `CI` que ponen GitHub
// Actions y casi todos los demás. Un `CI=false` o `CI=0` explícito cuenta como «no».
func enCI() bool {
	v := strings.TrimSpace(os.Getenv("CI"))
	return v != "" && v != "0" && !strings.EqualFold(v, "false")
}

// dibujoDelPanel es lo que el arnés de node contesta: el mapa INERTE_POR tal como lo evaluó la
// página, el HTML que automatico() devolvió por cada `inerte_por`, y el de una política sana.
type dibujoDelPanel struct {
	Error   string            `json:"error"`
	Textos  map[string]string `json:"textos"`
	Dibujos map[string]string `json:"dibujos"`
	Sana    string            `json:"sana"`
}

// automaticoEnNode evalúa `guion` en node y le pregunta a automatico() qué dibuja con cada freno de
// `frenos`, con cada clave de INERTE_POR y con claveQueElPanelNoConoce.
func automaticoEnNode(t *testing.T, guion string, frenos []string) dibujoDelPanel {
	t.Helper()
	dir := t.TempDir()
	arnes := filepath.Join(dir, "arnes-del-panel.js")
	if err := os.WriteFile(arnes, []byte(arnesDelPanel), 0o644); err != nil {
		t.Fatal(err)
	}
	// El pedido viaja en un ARCHIVO y no en la línea de comandos: un JSON con comillas pasado como
	// argumento se escapa distinto en cada sistema, y el job de Windows también corre esto.
	pedido, err := json.Marshal(map[string]any{"frenos": frenos, "desconocido": claveQueElPanelNoConoce})
	if err != nil {
		t.Fatal(err)
	}
	rutaPedido := filepath.Join(dir, "pedido.json")
	if err := os.WriteFile(rutaPedido, pedido, 0o644); err != nil {
		t.Fatal(err)
	}

	ctx, cancelar := context.WithTimeout(context.Background(), time.Minute)
	defer cancelar()
	cmd := guiones.HerramientaCtx(ctx, t, "node", arnes, rutaPedido)
	// NODE_OPTIONS vacío: un `--require` heredado del entorno correría código ajeno ANTES que la
	// página y podría cambiar lo que se mide.
	cmd.Env = append(os.Environ(), "NODE_OPTIONS=")
	cmd.Stdin = strings.NewReader(guion)
	var salida, errores bytes.Buffer
	cmd.Stdout, cmd.Stderr = &salida, &errores
	if err := cmd.Run(); err != nil {
		t.Fatalf("node no pudo evaluar el <script> de flota.html y preguntarle a automatico(): %v\n%s", err, errores.String())
	}
	var r dibujoDelPanel
	if err := json.Unmarshal(salida.Bytes(), &r); err != nil {
		t.Fatalf("el arnés de node no devolvió JSON (%v):\n%s\n%s", err, salida.String(), errores.String())
	}
	if r.Error != "" {
		t.Fatalf("flota.html no se deja medir: %s", r.Error)
	}
	return r
}

var (
	etiquetaHTML = regexp.MustCompile(`<[^>]*>`)
	tituloHTML   = regexp.MustCompile(`\stitle\s*=\s*(?:"([^"]*)"|'([^']*)')`)
)

// dibujadoDe devuelve lo que una persona puede LEER de un pedazo de HTML: el texto visible y los
// `title` de sus etiquetas —el motivo de una política inerte va en el title—, con las entidades
// resueltas. Un atributo que no se muestra (un `data-*`, una clase) no cuenta como dibujado.
func dibujadoDe(h string) string {
	var titulos []string
	for _, etiqueta := range etiquetaHTML.FindAllString(h, -1) {
		for _, m := range tituloHTML.FindAllStringSubmatch(etiqueta, -1) {
			titulos = append(titulos, html.UnescapeString(m[1]+m[2]))
		}
	}
	visible := strings.Join(strings.Fields(html.UnescapeString(etiquetaHTML.ReplaceAllString(h, " "))), " ")
	return strings.Join(append([]string{visible}, titulos...), "\n")
}

// arnesDelPanel es el lado de node de TestElPanelDibujaPorQueCadaPoliticaEstaInerte. Evalúa la
// página en un contexto aislado y DEVUELVE lo que automatico() dibuja: no juzga nada, las
// aserciones viven en Go. Va sin un solo backtick porque vive en un literal crudo de Go.
const arnesDelPanel = `'use strict';
const vm = require('vm');
const fs = require('fs');

const pagina = fs.readFileSync(0, 'utf8');
const pedido = JSON.parse(fs.readFileSync(process.argv[2], 'utf8'));

// Lo minimo del navegador para que la pagina DEFINA sus funciones sin correr ninguna: fetch no se
// resuelve nunca (el refresco del arranque queda en vuelo y no pinta) y los temporizadores no se
// programan. El resto es un sumidero que acepta cualquier acceso, para que un cambio inocente en
// el arranque de la pagina no ponga esto rojo por un motivo que no es el que se mide.
const sumidero = new Proxy(function () {}, {
  get: (_, k) => (k === 'then' ? undefined
    : k === Symbol.toPrimitive ? () => ''
    : k === Symbol.iterator ? function* () {}
    : sumidero),
  set: () => true,
  apply: () => sumidero,
  construct: () => sumidero,
});
const contexto = vm.createContext({
  document: sumidero, window: sumidero, location: sumidero, navigator: sumidero, CSS: sumidero,
  localStorage: sumidero, sessionStorage: sumidero, console: sumidero,
  fetch: () => new Promise(() => {}),
  setInterval: () => 0, setTimeout: () => 0, clearInterval: () => {}, clearTimeout: () => {},
  requestAnimationFrame: () => 0,
});
vm.runInContext(pagina, contexto, { filename: 'flota.html', timeout: 5000 });

// Corre ADENTRO del contexto: automatico e INERTE_POR son las de la pagina, no copias.
function preguntar() {
  if (typeof automatico !== 'function') {
    return JSON.stringify({ error: 'la pagina no define automatico()' });
  }
  if (typeof INERTE_POR !== 'object' || INERTE_POR === null) {
    return JSON.stringify({ error: 'la pagina no define el mapa INERTE_POR: volvio a un texto fijo, que es el defecto de A131' });
  }
  const textos = {};
  for (const k of Object.keys(INERTE_POR)) textos[k] = String(INERTE_POR[k]);
  const dibujar = extra => String(automatico({
    politicas_activas: 1,
    politicas: [Object.assign({
      nombre: 'vaciar-journal', principal: 'auto-heal', condicion: 'mem_pct > 90',
      hacer: ['journalctl', '--vacuum-size=200M'], cooldown_min: 60, ultimo_disparo: null,
    }, extra)],
  }));
  const claves = [...new Set([...Object.keys(textos), ...__pedido.frenos, __pedido.desconocido])];
  const dibujos = {};
  for (const k of claves) dibujos[k] = dibujar({ puede_actuar: false, inerte_por: k });
  return JSON.stringify({ textos, dibujos, sana: dibujar({ puede_actuar: true }) });
}
contexto.__pedido = pedido;
process.stdout.write(vm.runInContext('(' + preguntar.toString() + ')()', contexto, { filename: 'arnes-del-panel', timeout: 5000 }));
`
