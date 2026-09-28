package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"musubi/internal/config"
	"musubi/internal/guiones"
)

// latidoApagaAMasTardar es lo que el latido del riel PROMETE ante un corte real: la lámpara se
// apaga a más tardar a los 10 min del último sondeo. Vive acá y NO se lee de latido.mjs: si la
// prueba tomara el umbral del módulo, cambiar el umbral movería la vara junto con lo medido.
const latidoApagaAMasTardar = 10 * time.Minute

// formatoAtDelFeed es el `at` de un LiveEvent tal como lo estampa el cerebro (publicarUso, en
// internal/mcp/livefeed.go).
const formatoAtDelFeed = "2006-01-02T15:04:05.000Z07:00"

// EL LATIDO DEL RIEL NO DICE «SIN SONDEO» CON EL SISTEMA SANO, Y SE APAGA ANTE UN CORTE REAL.
//
// El pie del riel en vivo contaba los sondeos del último minuto y con cero decía «sin sondeo».
// Desde la bajada espaciada (internal/mcp/bajada_ritmo.go) una máquina quieta pide cada 300 s, y
// hasta ~450 s cuando el candado de la bajada cambia de dueño. Con los 3 sondeadores que el central
// vio en 24 h (medido el 2026-09-27), la ventana de un minuto apagaba la lámpara el 51,6 % del
// tiempo con todo sano (simulado con fases al azar; el 80 % con un solo sondeador). Ahora lo decide
// assets/src/latido.mjs: prendida mientras el último sondeo tenga 10 min o menos, el texto dice
// hace cuánto llegó, y la hora es la del `at` del evento, no la de su llegada al navegador.
//
// LO QUE SE EXIGE, CONTRA LOS NÚMEROS DEL RITMO Y NO CONTRA LA CONSTANTE DEL PANEL:
//
//  1. Un hueco sano no la apaga: ni el tope del espaciado repetido una hora, ni el peor hueco
//     (peorHuecoSanoDeLaBajada, que sale de la config), ni un sondeo de hace una hora que el relay
//     re-manda al reconectar.
//  2. Un corte la apaga DESPUÉS del peor hueco sano y a más tardar a los 10 min (+1 s de muestreo)
//     del último sondeo, también con el reloj del central adelantado, y no se vuelve a prender sola.
//  3. El backlog que el relay manda al abrir la página no la prende con sondeos viejos.
//  4. El texto nunca dice «sin sondeo» con la lámpara prendida, y la antigüedad que da es la real.
//
// Si mañana sube el tope por defecto del ritmo, esto se pone rojo solo: el umbral del panel tiene
// que seguirlo, y la prueba lo dice antes de que lo diga la lámpara.
//
// SE EJECUTA EN NODE, como TestElPanelDibujaPorQueCadaPoliticaEstaInerte y por el mismo motivo: lo
// que importa es lo que el latido DICE segundo a segundo, no cómo está escrito. Corre el FUENTE
// (assets/src/latido.mjs), que no se embebe: el bundle que sí se sirve sale de ese fuente, y el job
// `panel` de ci.yml falla si el bundle commiteado no es el que da `npm run build`. El cableado de
// dashboard.mjs —quién llama a anotarSondeo y a latido— queda fuera de esta prueba. Sin node, en
// CI es un fallo y afuera un salteo que dice por qué.
//
// Sabotaje: devolver el umbral a la ventana vieja de un minuto: con un sondeo cada 5 min la
// lámpara se apaga cuatro de cada cinco minutos. La otra dirección: un umbral de 8 min también le
// gana al peor hueco sano y cumple los 10 min, y tiene que seguir en verde.
// arnes: archivo="cmd/musubi/assets/src/latido.mjs"
// arnes: de="export const UMBRAL_LATIDO_MS = 10 * 60 * 1000;"
// arnes: a="export const UMBRAL_LATIDO_MS = 60 * 1000;"
// arnes: arreglo_de="export const UMBRAL_LATIDO_MS = 10 * 60 * 1000;"
// arnes: arreglo_a="export const UMBRAL_LATIDO_MS = 8 * 60 * 1000;"
//
// Sabotaje: una lámpara que no se apaga nunca: veinte minutos sin un solo sondeo y el pie sigue
// diciendo que la flota sondea.
// arnes: archivo="cmd/musubi/assets/src/latido.mjs"
// arnes: de="const vive = edad <= UMBRAL_LATIDO_MS;"
// arnes: a="const vive = true;"
//
// Sabotaje: estampar el sondeo a su LLEGADA e ignorar su `at`, que es la conducta vieja: el
// backlog de hace 20 min prende la lámpara al abrir la página. La otra dirección: leer la hora con
// `new Date(at).getTime()` da lo mismo y tiene que seguir en verde.
// arnes: archivo="cmd/musubi/assets/src/latido.mjs"
// arnes: de="const t = Date.parse(at);"
// arnes: a="const t = NaN;"
// arnes: arreglo_de="const t = Date.parse(at);"
// arnes: arreglo_a="const t = new Date(at).getTime();"
//
// Sabotaje: intercambiar los dos textos: con la lámpara prendida el pie dice «sin sondeo desde
// hace 40s», que es el defecto de este arreglo en su forma más literal.
// arnes: archivo="cmd/musubi/assets/src/latido.mjs"
// arnes: de="texto: vive ? 'sondeo · hace '"
// arnes: a="texto: !vive ? 'sondeo · hace '"
//
// Sabotaje: no recortar un `at` del futuro: con el reloj del central 30 s adelantado, un corte
// apaga la lámpara 30 s después de lo prometido.
// arnes: archivo="cmd/musubi/assets/src/latido.mjs"
// arnes: de="Number.isFinite(t) ? Math.min(t, llegada) : llegada;"
// arnes: a="Number.isFinite(t) ? t : llegada;"
//
// Sabotaje: dejar que la marca retroceda: un sondeo de hace una hora que el relay re-manda al
// reconectar apaga la lámpara con el sistema sano.
// arnes: archivo="cmd/musubi/assets/src/latido.mjs"
// arnes: de="return Math.max(Number(ultimo) || 0, marca);"
// arnes: a="return marca;"
func TestElLatidoAguantaLaBajadaEspaciadaYSeApagaConUnCorte(t *testing.T) {
	nodeParaElLatido(t)

	tope := config.Default().Sync.EffectiveInboundIdleMaxSeconds()
	peor := peorHuecoSanoDeLaBajada()
	promesa := int(latidoApagaAMasTardar / time.Second)
	if peor >= promesa {
		t.Fatalf("el peor hueco sano de la bajada (%d s, con un tope de %d s) ya no entra en los %d s que "+
			"promete el latido: con el sistema sano la lámpara se apagaría. El umbral de latido.mjs y esta "+
			"promesa suben juntos, o el tope del ritmo baja", peor, tope, promesa)
	}

	// El `at` va en otra zona que la del navegador, como puede venir del central: leerlo mal corre
	// la marca horas enteras.
	t0 := time.Date(2026, 9, 27, 12, 0, 0, 0, time.UTC)
	zona := time.FixedZone("central", -3*3600)
	at := func(seg int) string {
		return t0.Add(time.Duration(seg) * time.Second).In(zona).Format(formatoAtDelFeed)
	}
	// enHora: sondeos cuyo `at` es su propia llegada, que es lo normal.
	enHora := func(llegadas ...int) []sondeoDeLatido {
		out := make([]sondeoDeLatido, 0, len(llegadas))
		for _, l := range llegadas {
			out = append(out, sondeoDeLatido{Llegada: l, At: at(l)})
		}
		return out
	}
	cada := func(paso, hasta int) []int {
		var out []int
		for s := 0; s <= hasta; s += paso {
			out = append(out, s)
		}
		return out
	}

	escenarios := []escenarioDeLatido{
		{Nombre: "ritmo", Sondeos: enHora(cada(tope, 3600)...), Hasta: 3600},
		{Nombre: "relevo", Sondeos: enHora(0, tope, tope+peor, 2*tope+peor), Hasta: 2*tope + peor},
		{Nombre: "reconexion", Sondeos: append(enHora(cada(tope, 1800)...),
			sondeoDeLatido{Llegada: 1000, At: at(-3600)}), Hasta: 1800},
		{Nombre: "corte", Sondeos: enHora(0, tope, 2*tope), Hasta: 2*tope + 2*promesa},
		{Nombre: "adelantado", Sondeos: []sondeoDeLatido{{Llegada: 0, At: at(30)}}, Hasta: 2 * promesa},
		{Nombre: "backlog", Sondeos: []sondeoDeLatido{{Llegada: 0, At: at(-1200)}}, Hasta: 10},
		{Nombre: "nunca", Hasta: 5},
	}
	r := latidoEnNode(t, t0, escenarios)

	// PISOS: sin todas las muestras, lo de abajo mediría menos de lo que dice, y «ningún segundo
	// apagado» sobre cero segundos es verde sin haber mirado nada.
	for _, e := range escenarios {
		m := r[e.Nombre]
		if len(m) != e.Hasta-e.Desde+1 {
			t.Fatalf("node devolvió %d muestras del escenario %q y se pidieron %d", len(m), e.Nombre, e.Hasta-e.Desde+1)
		}
		for i, x := range m {
			if x.T != e.Desde+i {
				t.Fatalf("la muestra %d del escenario %q es del segundo %d y tenía que ser del %d", i, e.Nombre, x.T, e.Desde+i)
			}
		}
	}

	// 1. UN HUECO SANO NO LA APAGA.
	for _, e := range []struct{ nombre, que string }{
		{"ritmo", fmt.Sprintf("un sondeo cada %d s durante una hora (el tope del espaciado de la bajada)", tope)},
		{"relevo", fmt.Sprintf("un hueco de %d s (el peor sano: el candado de la bajada cambia de dueño)", peor)},
		{"reconexion", "un sondeo de hace una hora que el relay re-manda al reconectar, en medio del ritmo"},
	} {
		if n, primero := contarApagados(r[e.nombre]); n > 0 {
			t.Errorf("con %s la lámpara se apagó %d de %d segundos, el primero a los %d s, con el sistema sano: "+
				"es el «sin sondeo» de más que este arreglo vino a sacar", e.que, n, len(r[e.nombre]), primero)
		}
	}

	// 2. UN CORTE LA APAGA ENTRE EL PEOR HUECO SANO Y LA PROMESA, Y NO SE PRENDE SOLA.
	for _, e := range []struct {
		nombre, que string
		ultimo      int
	}{
		{"corte", "después de tres sondeos en ritmo", 2 * tope},
		{"adelantado", "con el reloj del central 30 s adelantado respecto del navegador", 0},
	} {
		primero, reprendida := -1, -1
		for _, x := range r[e.nombre] {
			if x.T < e.ultimo {
				continue
			}
			if !x.Vive && primero < 0 {
				primero = x.T
			}
			if primero >= 0 && x.Vive && reprendida < 0 {
				reprendida = x.T
			}
		}
		switch d := primero - e.ultimo; {
		case primero < 0:
			t.Errorf("un corte de %d s %s no apagó la lámpara nunca: el pie dice que la flota sondea y nadie "+
				"sondea", 2*promesa, e.que)
		case d <= peor:
			t.Errorf("un corte %s apagó la lámpara a los %d s del último sondeo, sin llegar al peor hueco sano "+
				"(%d s): con el sistema sano también se apagaría", e.que, d, peor)
		case d > promesa+1:
			t.Errorf("un corte %s apagó la lámpara recién a los %d s del último sondeo, y la promesa es "+
				"apagarla a los %d s", e.que, d, promesa)
		case reprendida >= 0:
			t.Errorf("un corte %s apagó la lámpara a los %d s y volvió a prenderse a los %d s sin ningún sondeo "+
				"nuevo", e.que, primero, reprendida)
		}
	}

	// 3. EL BACKLOG VIEJO NO LA PRENDE, Y SIN NINGÚN SONDEO QUEDA APAGADA.
	for _, e := range []struct{ nombre, que string }{
		{"backlog", "un sondeo de hace 20 min que llega con el backlog al abrir la página"},
		{"nunca", "ningún sondeo"},
	} {
		if apagados, _ := contarApagados(r[e.nombre]); apagados < len(r[e.nombre]) {
			t.Errorf("con %s la lámpara quedó prendida %d de %d segundos: el panel afirma un sondeo que no está "+
				"pasando", e.que, len(r[e.nombre])-apagados, len(r[e.nombre]))
		}
	}

	// 4. EL TEXTO: nunca «sin sondeo» con la lámpara prendida, y la antigüedad del último sondeo
	// real. La marca esperada se calcula acá: la hora del evento, recortada a su llegada si viene
	// del futuro, y sin retroceder nunca. Si la lámpara tenía que estar prendida lo miden 1 a 3;
	// acá se mira que el texto diga lo mismo que la lámpara.
	for _, e := range escenarios {
		for _, x := range r[e.Nombre] {
			edad, hay := edadEsperada(t, t0, e.Sondeos, x.T)
			quiero := "sin sondeo"
			switch {
			case hay && x.Vive:
				quiero = "sondeo · hace " + antiguedadEsperada(edad)
			case hay:
				quiero = "sin sondeo desde hace " + antiguedadEsperada(edad)
			}
			if x.Texto != quiero {
				t.Errorf("escenario %q, segundo %d, lámpara prendida=%v: el pie dice %q y tenía que decir %q",
					e.Nombre, x.T, x.Vive, x.Texto, quiero)
				break // una por escenario: el resto repite la misma
			}
		}
	}
}

// peorHuecoSanoDeLaBajada es el hueco más largo entre dos sondeos con el sistema SANO, en segundos:
// el tope del espaciado (una máquina quieta pide una vez por tope), más el lease del candado de la
// bajada y un tick, que es lo que tarda otro proceso en tomarla cuando el dueño se muere sin
// soltarla (leaseBajadaSegundos en internal/mcp/scheduler.go: cuatro ticks, con piso de 120 s).
// Con la config por defecto, 300 + 120 + 30 = 450 s.
func peorHuecoSanoDeLaBajada() int {
	tope := config.Default().Sync.EffectiveInboundIdleMaxSeconds()
	tick := config.Default().Sync.DrainIntervalSeconds
	return tope + max(4*tick, 120) + tick
}

// sondeoDeLatido es un sondeo que le llega al panel: a qué segundo del escenario llega y el `at`
// que trae.
type sondeoDeLatido struct {
	Llegada int    `json:"llegada"`
	At      string `json:"at"`
}

// escenarioDeLatido es una secuencia de sondeos y los segundos en que se mira la lámpara.
type escenarioDeLatido struct {
	Nombre  string           `json:"nombre"`
	Sondeos []sondeoDeLatido `json:"sondeos"`
	Desde   int              `json:"desde"`
	Hasta   int              `json:"hasta"`
}

// muestraDeLatido es lo que el latido dijo en un segundo.
type muestraDeLatido struct {
	T     int    `json:"t"`
	Vive  bool   `json:"vive"`
	Texto string `json:"texto"`
}

// contarApagados cuenta los segundos con la lámpara apagada y devuelve el primero (-1 si ninguno).
func contarApagados(m []muestraDeLatido) (n, primero int) {
	primero = -1
	for _, x := range m {
		if !x.Vive {
			if primero < 0 {
				primero = x.T
			}
			n++
		}
	}
	return n, primero
}

// edadEsperada es la antigüedad, en ms, del último sondeo que el panel vio hasta el segundo `seg`:
// la hora de cada evento es su `at`, recortada a la llegada si viene del futuro, y la marca nunca
// retrocede. `hay` es false si todavía no llegó ninguno.
func edadEsperada(t *testing.T, t0 time.Time, sondeos []sondeoDeLatido, seg int) (edad int64, hay bool) {
	t.Helper()
	ahora := t0.Add(time.Duration(seg) * time.Second).UnixMilli()
	var marca int64
	for _, s := range sondeos {
		llegada := t0.Add(time.Duration(s.Llegada) * time.Second).UnixMilli()
		if llegada > ahora {
			continue
		}
		cuando, err := time.Parse(formatoAtDelFeed, s.At)
		if err != nil {
			t.Fatalf("el `at` %q del escenario no se lee con el formato del feed: %v", s.At, err)
		}
		// Sin el min/max de Go a propósito: este paquete de pruebas define su propio `min` de
		// enteros (agentskills_test.go), que tapa al del lenguaje.
		hora := cuando.UnixMilli()
		if hora > llegada {
			hora = llegada
		}
		if hora > marca {
			marca = hora
		}
		hay = true
	}
	if edad = ahora - marca; edad < 0 {
		edad = 0
	}
	return edad, hay
}

// antiguedadEsperada es la antigüedad en las unidades gruesas del riel (s → m → h), redondeada
// como Math.round en el panel.
func antiguedadEsperada(ms int64) string {
	s := int64(math.Round(float64(ms) / 1000))
	if s < 60 {
		return fmt.Sprintf("%ds", s)
	}
	m := int64(math.Round(float64(s) / 60))
	if m < 60 {
		return fmt.Sprintf("%dm", m)
	}
	return fmt.Sprintf("%dh", int64(math.Round(float64(m)/60)))
}

// nodeParaElLatido exige node para ejecutar latido.mjs: en CI su ausencia es un FALLO, y afuera
// un salteo con el motivo. Es la regla de nodeParaElPanel, con el motivo de esta prueba.
func nodeParaElLatido(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("node"); err == nil {
		return
	}
	if enCI() {
		t.Fatalf("no hay `node` en el PATH y esto corre en CI (CI=%q): la guarda del latido no puede ejecutar "+
			"latido.mjs, y un salteo acá sería «no medí» con la cara de «medí y está bien»", os.Getenv("CI"))
	}
	t.Skip("SALTEADA: no hay `node` en el PATH. Esta prueba EJECUTA assets/src/latido.mjs para ver qué dice " +
		"el latido del riel segundo a segundo, y sin node no puede. En CI (con la variable CI puesta) esto es " +
		"un FALLO y no un salteo. Instalá node para medirla acá.")
}

// latidoEnNode corre los escenarios contra latido.mjs en node y devuelve, por escenario, lo que el
// latido dijo en cada segundo.
func latidoEnNode(t *testing.T, t0 time.Time, escenarios []escenarioDeLatido) map[string][]muestraDeLatido {
	t.Helper()
	modulo, err := filepath.Abs(filepath.Join("assets", "src", "latido.mjs"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(modulo); err != nil {
		t.Fatalf("no está el latido del panel donde lo busca esta prueba: %v", err)
	}
	dir := t.TempDir()
	arnes := filepath.Join(dir, "arnes-del-latido.mjs")
	if err := os.WriteFile(arnes, []byte(arnesDelLatido), 0o644); err != nil {
		t.Fatal(err)
	}
	// El pedido viaja en un ARCHIVO y no en la línea de comandos, como en automaticoEnNode.
	pedido, err := json.Marshal(map[string]any{"t0": t0.UnixMilli(), "escenarios": escenarios})
	if err != nil {
		t.Fatal(err)
	}
	rutaPedido := filepath.Join(dir, "pedido.json")
	if err := os.WriteFile(rutaPedido, pedido, 0o644); err != nil {
		t.Fatal(err)
	}

	ctx, cancelar := context.WithTimeout(context.Background(), time.Minute)
	defer cancelar()
	cmd := guiones.HerramientaCtx(ctx, t, "node", arnes, modulo, rutaPedido)
	// NODE_OPTIONS vacío: un `--require` heredado correría código ajeno antes que el módulo.
	cmd.Env = append(os.Environ(), "NODE_OPTIONS=")
	var salida, errores bytes.Buffer
	cmd.Stdout, cmd.Stderr = &salida, &errores
	if err := cmd.Run(); err != nil {
		t.Fatalf("node no pudo correr latido.mjs: %v\n%s", err, errores.String())
	}
	var r struct {
		Error      string                       `json:"error"`
		Escenarios map[string][]muestraDeLatido `json:"escenarios"`
	}
	if err := json.Unmarshal(salida.Bytes(), &r); err != nil {
		t.Fatalf("el arnés de node no devolvió JSON (%v):\n%s\n%s", err, salida.String(), errores.String())
	}
	if r.Error != "" {
		t.Fatalf("latido.mjs no se deja medir: %s", r.Error)
	}
	return r.Escenarios
}

// arnesDelLatido es el lado de node de TestElLatidoAguantaLaBajadaEspaciadaYSeApagaConUnCorte:
// importa latido.mjs tal como está en el árbol, recorre cada escenario segundo a segundo como lo
// hace tictac() en el panel, y DEVUELVE lo que dijo el latido. No juzga nada: las aserciones viven
// en Go. Va sin un solo backtick porque vive en un literal crudo de Go.
const arnesDelLatido = `import { readFileSync } from 'node:fs';
import { pathToFileURL } from 'node:url';

async function medir() {
  let latido;
  try {
    latido = await import(pathToFileURL(process.argv[2]).href);
  } catch (e) {
    return { error: 'node no pudo importar latido.mjs: ' + (e && e.stack ? e.stack : e) };
  }
  if (typeof latido.anotarSondeo !== 'function' || typeof latido.latido !== 'function') {
    return { error: 'latido.mjs no exporta anotarSondeo() y latido(), que son las que usa el panel' };
  }
  const pedido = JSON.parse(readFileSync(process.argv[3], 'utf8'));
  const S = 1000;
  const escenarios = {};
  for (const e of pedido.escenarios) {
    // Cada sondeo llega a su hora y se anota con el reloj del navegador de ese momento, como en
    // anotarEvento(); la lampara se mira una vez por segundo, como en tictac().
    const pendientes = (e.sondeos || [])
      .map((s) => ({ llegada: pedido.t0 + s.llegada * S, at: s.at }))
      .sort((a, b) => a.llegada - b.llegada);
    const muestras = [];
    let ultimo = 0;
    let i = 0;
    for (let seg = e.desde; seg <= e.hasta; seg++) {
      const ahora = pedido.t0 + seg * S;
      while (i < pendientes.length && pendientes[i].llegada <= ahora) {
        const s = pendientes[i++];
        ultimo = latido.anotarSondeo(ultimo, s.at, s.llegada);
      }
      const l = latido.latido(ultimo, ahora) || {};
      muestras.push({ t: seg, vive: Boolean(l.vive), texto: String(l.texto) });
    }
    escenarios[e.nombre] = muestras;
  }
  return { escenarios };
}

process.stdout.write(JSON.stringify(await medir()));
`
