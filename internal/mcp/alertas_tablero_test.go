package mcp

// alertas_tablero_test.go — las guardas de deploy/musubi-alerts-tablero.yml (punto 32 de la ola 1).
//
// No hay `promtool` en esta PC ni en la CI, así que la semántica de estas reglas no se puede
// ejecutar acá: se valida en el despliegue (`podman exec musubi-prometheus promtool check rules`).
// Lo que sí se puede custodiar en el repo es la FORMA de la expresión, que es justo donde estaba el
// defecto: las tres reglas del plan original se leían bien y dos no podían dispararse nunca.

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

const archivoDeAlertasDelTablero = "musubi-alerts-tablero.yml"

// cadenciaDelTablero es cada cuánto el tablero barre y empuja una foto: publicar.log en davantis-1
// tiene una corrida a los :13, :28, :43 y :58 de cada hora. Todas las ventanas se miden contra ella.
const cadenciaDelTablero = 15 * time.Minute

// alertasDelTablero devuelve las alertas del archivo del tablero, leídas por el MISMO glob que
// usan Prometheus y el verificador. Si el archivo sale del glob, esto no lo encuentra y lo dice.
func alertasDelTablero(t *testing.T) map[string]alertaCruda {
	t.Helper()
	out := map[string]alertaCruda{}
	for _, a := range alertasCrudasDelRepo(t) {
		if a.Archivo == archivoDeAlertasDelTablero {
			a.Expr = strings.Join(strings.Fields(a.Expr), " ")
			out[a.Nombre] = a
		}
	}
	// EL CONTROL VA PRIMERO: sin alertas, cada guarda de abajo pasaría sin haber mirado nada.
	for _, nombre := range []string{"FlotaSinFoto", "FlotaRinconCiego", "FlotaPiezaCaida"} {
		if _, hay := out[nombre]; !hay {
			t.Fatalf("no encontré %s en deploy/%s (leí %d alertas de ese archivo).\n"+
				"  O se renombró, o el archivo salió del glob `musubi-alerts*.yml` —y entonces ni "+
				"Prometheus ni verificar-despliegue.sh lo ven—. Un verde acá no significaría nada.",
				nombre, archivoDeAlertasDelTablero, len(out))
		}
	}
	return out
}

// TestLasAlertasDelTableroLeenSobreUnRango — la hermana de
// TestLasAlertasDelLatidoLeenLaSerieConLastOverTime, para una serie que llega cada 15 minutos.
//
// UNA SERIE EMPUJADA CADA 15 MIN VIVE 5 EN EL VECTOR INSTANTÁNEO. El lookback del Prometheus del
// server es `query.lookback-delta: 5m` (medido con /api/v1/status/flags el 2026-09-24): los otros
// diez minutos, la métrica pelada da VACÍO. El plan original escribía
// `flota_rincon_alcanzado == 0` con `for: 30m`, y eso NO PUEDE DISPARARSE NUNCA: el `for` se
// reinicia cada 15 min y no llega a los 30. Leída en el YAML, la regla dice exactamente lo que uno
// quiere que diga; por eso la guarda es sobre la forma y no sobre el sentido.
//
// Y EL RANGO TIENE QUE CUBRIR DOS FOTOS CON MARGEN, no una: con `[15m]` una foto que llega con
// segundos de atraso deja la ventana vacía, y un `absent_over_time` de ese largo dispararía en
// falso; un `count_over_time >= 2` sobre él no podría cumplirse nunca.
//
// Sabotaje que la hace fallar: volver a la métrica pelada.
// arnes: archivo="deploy/musubi-alerts-tablero.yml"
// arnes: de="max_over_time(flota_rincon_alcanzado[35m]) == 0"
// arnes: a="flota_rincon_alcanzado == 0"
// Sabotaje que la hace fallar: una ventana de una sola cadencia.
// arnes: archivo="deploy/musubi-alerts-tablero.yml"
// arnes: de="absent_over_time(flota_foto_cuando_segundos[40m])"
// arnes: a="absent_over_time(flota_foto_cuando_segundos[15m])"
func TestLasAlertasDelTableroLeenSobreUnRango(t *testing.T) {
	alertas := alertasDelTablero(t)

	reCruda := regexp.MustCompile(`\bflota_[a-z0-9_]+`)
	// La forma buena: CUALQUIER `*_over_time(` con la métrica adentro y un rango. Se tachan las
	// envueltas y se mira si sobra alguna pelada: contar unas y otras por separado daría verde en
	// una expresión con una de cada.
	reEnvuelta := regexp.MustCompile(`\b[a-z_]+_over_time\(\s*flota_[a-z0-9_]+\s*(?:\{[^}]*\})?\s*\[([^\]]+)\]`)

	vistas := 0
	for nombre, a := range alertas {
		crudas := reCruda.FindAllString(a.Expr, -1)
		envueltas := reEnvuelta.FindAllStringSubmatch(a.Expr, -1)
		vistas += len(crudas)
		if len(envueltas) < len(crudas) {
			t.Errorf("%s lee una serie del tablero SIN `*_over_time(...[rango])`:\n  %s\n"+
				"La foto llega cada 15 min y en el vector instantáneo vive 5: los otros 10 la "+
				"expresión da vacío. Con un `for` más largo que eso la alerta no dispara nunca; sin "+
				"`for`, dispara y se resuelve en cada barrido. Envolvela: `max_over_time(<metrica>[35m])`.",
				nombre, a.Expr)
		}
		for _, m := range envueltas {
			rango, err := duracionProm(m[1])
			if err != nil {
				t.Errorf("%s: no entiendo el rango %q: %v", nombre, m[1], err)
				continue
			}
			if rango <= 2*cadenciaDelTablero {
				t.Errorf("%s lee sobre [%s], y el tablero empuja cada %s: la ventana tiene que "+
					"cubrir DOS fotos con margen (más de %s). Con menos, una foto que llega tarde la "+
					"deja vacía — y ahí un `absent_over_time` suena en falso y un `count_over_time >= 2` "+
					"no se cumple nunca.", nombre, m[1], cadenciaDelTablero, 2*cadenciaDelTablero)
			}
		}
	}
	if vistas < 3 {
		t.Fatalf("sólo vi %d lecturas de series `flota_*` en las expresiones del tablero: el detector "+
			"dejó de mirar, o las alertas dejaron de leer lo que el tablero empuja", vistas)
	}
}

// TestUnaSolaFotoNoAlcanzaParaAvisar — la persistencia la exige el conteo, no un `for`.
//
// MEDIDO sobre las 1785 fotos publicadas del 2026-08-17 al 2026-09-24: las caídas subieron 146
// veces y 113 duraron UNA sola foto; de 18 rincones ciegos, 9. Una alerta que dispare con una foto
// son 146 avisos en 38 días, y un canal así se aprende a ignorar.
//
// `max_over_time(...[35m]) == 0` sola NO alcanza para eso: con una serie recién nacida, o con un
// empuje perdido, la ventana tiene UNA muestra y el máximo de una muestra ciega es cero. Por eso
// toda lectura de un estado del tablero va acompañada de `count_over_time(<la misma>[…]) >= 2`.
//
// Sabotaje que la hace fallar: conformarse con una foto.
// arnes: archivo="deploy/musubi-alerts-tablero.yml"
// arnes: de="count_over_time(flota_pieza_caida[35m]) >= 2"
// arnes: a="count_over_time(flota_pieza_caida[35m]) >= 1"
// Sabotaje que la hace fallar: contar OTRA serie que la que se lee.
// arnes: archivo="deploy/musubi-alerts-tablero.yml"
// arnes: de="count_over_time(flota_rincon_alcanzado[35m]) >= 2"
// arnes: a="count_over_time(flota_foto_cuando_segundos[35m]) >= 2"
func TestUnaSolaFotoNoAlcanzaParaAvisar(t *testing.T) {
	alertas := alertasDelTablero(t)

	// Lo que lee un ESTADO: el máximo, el mínimo, el último. `absent_over_time` no entra: el hombre
	// muerto pregunta por la ausencia, y ahí no hay fotos que contar.
	reLectura := regexp.MustCompile(`\b(?:max|min|last|avg)_over_time\(\s*(flota_[a-z0-9_]+)`)

	revisadas := 0
	for nombre, a := range alertas {
		for _, m := range reLectura.FindAllStringSubmatch(a.Expr, -1) {
			serie := m[1]
			revisadas++
			reCuenta := regexp.MustCompile(`\bcount_over_time\(\s*` + regexp.QuoteMeta(serie) +
				`\s*(?:\{[^}]*\})?\s*\[[^\]]+\]\s*\)\s*>=\s*(\d+)`)
			c := reCuenta.FindStringSubmatch(a.Expr)
			if c == nil {
				t.Errorf("%s lee %s y no exige cuántas fotos lo confirman (`count_over_time(%s[…]) >= 2`).\n"+
					"  Con una serie recién nacida o un empuje perdido, la ventana tiene UNA muestra y "+
					"esa sola decide: 113 de 146 caídas medidas duraron una foto.", nombre, serie, serie)
				continue
			}
			if n, _ := strconv.Atoi(c[1]); n < 2 {
				t.Errorf("%s se conforma con %d foto(s) de %s: una sola foto es el pico que esta "+
					"regla existe para no avisar (113 de 146 caídas medidas).", nombre, n, serie)
			}
		}
	}
	// FlotaRinconCiego y FlotaPiezaCaida leen un estado cada una.
	if revisadas < 2 {
		t.Fatalf("sólo encontré %d lectura(s) de un estado del tablero (`max/min_over_time(flota_…)`) y "+
			"son al menos dos: el detector dejó de mirar y un verde acá no diría nada", revisadas)
	}
}

// TestElTableroSeCallaCuandoLaPcQueLoPublicaEstaApagada — la decisión del dueño (2026-09-24).
//
// El tablero corre en davantis-1, y esa PC se apaga: sin ella no hay foto, y eso ya lo avisa
// `MaquinaCaida{device="davantis-1"}`. Sin el `unless`, `FlotaSinFoto` sonaría CADA noche; y sin el
// `for`, sonaría y se resolvería en cada ARRANQUE, porque al volver la PC la última foto tiene horas
// y la primera nueva tarda. Medido cruzando las subidas 0→1 de
// `musubi_fleet_device_up{device="davantis-1"}` con publicar.log (2026-09-12 a 09-24): con el
// publicador sano, la primera foto llegó entre 0,8 y 7,1 min después; en teoría, una cadencia.
//
// La otra mitad es `FlotaRinconCiego`: `gio` lleva días con `MaquinaCaida` activa y el tablero la
// ve ciega en cada barrido. El `unless` va por `on(device)` porque los `project` difieren por
// construcción (`musubi` contra `flota`), y una inhibición de Alertmanager con `equal: ['project',
// 'device']` no los emparejaría nunca.
//
// Sabotaje que la hace fallar: que la PC apagada no calle el hombre muerto.
// arnes: archivo="deploy/musubi-alerts-tablero.yml"
// arnes: de="(last_over_time(musubi_fleet_device_up{device=\"davantis-1\",instance=\"musubi-otlp-push\"}[10m]) == 0)"
// arnes: a="(last_over_time(musubi_fleet_device_up{device=\"davantis-1\",instance=\"musubi-otlp-push\"}[10m]) == 2)"
// Sabotaje que la hace fallar: sin `for`, suena en cada arranque de la PC.
// arnes: archivo="deploy/musubi-alerts-tablero.yml"
// arnes: de="        for: 20m\n"
// arnes: a="        for: 0m\n"
// Sabotaje que la hace fallar: el rincón de una máquina que Musubi ya da por caída avisa dos veces.
// arnes: archivo="deploy/musubi-alerts-tablero.yml"
// arnes: de="(last_over_time(musubi_fleet_device_up{instance=\"musubi-otlp-push\"}[10m]) == 0)"
// arnes: a="(last_over_time(musubi_fleet_device_up{instance=\"musubi-otlp-push\"}[10m]) == 2)"
func TestElTableroSeCallaCuandoLaPcQueLoPublicaEstaApagada(t *testing.T) {
	alertas := alertasDelTablero(t)

	// SE COMPARA EL TEXTO DE LA CONDICIÓN, con los espacios normalizados. Es una decisión con nombre
	// —qué máquina publica y qué serie dice que está apagada—, y cualquier reescritura tiene que
	// pasar por acá y decir por qué.
	callarLaPc := `unless on() (last_over_time(musubi_fleet_device_up{device="davantis-1",instance="musubi-otlp-push"}[10m]) == 0)`
	if a := alertas["FlotaSinFoto"]; !strings.Contains(a.Expr, callarLaPc) {
		t.Errorf("FlotaSinFoto ya no se calla con davantis-1 apagada:\n  %s\n"+
			"  Esperaba `%s`. Sin eso suena cada noche que esa PC se apaga, cuando MaquinaCaida ya "+
			"lo está diciendo — y un aviso doble cada noche enseña a ignorar el canal.",
			a.Expr, callarLaPc)
	}

	a := alertas["FlotaSinFoto"]
	plazo, err := duracionProm(a.Plazo)
	if !a.TienePlazo || err != nil || plazo <= cadenciaDelTablero {
		t.Errorf("FlotaSinFoto tiene `for: %s`, y tiene que ser más largo que una cadencia (%s).\n"+
			"  Al prender davantis-1 el `unless` deja de callar y la última foto tiene horas: la "+
			"alerta se pone en pie antes de que llegue la primera foto nueva, que puede tardar una "+
			"cadencia entera. Sin ese margen suena y se resuelve en CADA arranque.",
			a.Plazo, cadenciaDelTablero)
	}

	callarCaidas := `unless on(device) (last_over_time(musubi_fleet_device_up{instance="musubi-otlp-push"}[10m]) == 0)`
	if a := alertas["FlotaRinconCiego"]; !strings.Contains(a.Expr, callarCaidas) {
		t.Errorf("FlotaRinconCiego ya no se calla sobre las máquinas que Musubi da por caídas:\n  %s\n"+
			"  Esperaba `%s`. Sin eso, gio —con MaquinaCaida activa hace días— avisa además como "+
			"rincón ciego en cada barrido, para siempre.", a.Expr, callarCaidas)
	}
}

// TestLaPruebaDePromtoolDelTableroCubreSusTresAlertas — lo que ata la prueba de promtool al repo.
//
// deploy/musubi-alerts-tablero.promtool es la ÚNICA comprobación que ejecuta estas reglas, y no la
// corre la CI (no hay promtool): se corre en el despliegue. Una prueba que sólo corre a mano se
// pudre sin que nadie lo note —se renombra una alerta, y el caso que la probaba queda probando un
// nombre que ya no existe, en verde—. Esta guarda es la mitad que sí corre: exige que la prueba
// cargue ESTE archivo de reglas y que cada alerta tenga un caso que la dispara y otro que no.
//
// Sabotaje que la hace fallar: que el caso que dispara FlotaSinFoto pruebe otro nombre.
// arnes: archivo="deploy/musubi-alerts-tablero.promtool"
// arnes: de="      - eval_time: 65m\n        alertname: FlotaSinFoto\n        exp_alerts:\n          - exp_labels"
// arnes: a="      - eval_time: 65m\n        alertname: FlotaSinFotoVieja\n        exp_alerts:\n          - exp_labels"
func TestLaPruebaDePromtoolDelTableroCubreSusTresAlertas(t *testing.T) {
	alertas := alertasDelTablero(t)
	ruta := filepath.Join("..", "..", "deploy", "musubi-alerts-tablero.promtool")
	crudo, err := os.ReadFile(ruta)
	if err != nil {
		t.Fatalf("falta %s: %v — sin ella, nada ejecuta estas reglas antes de producción", ruta, err)
	}
	var prueba struct {
		RuleFiles []string `yaml:"rule_files"`
		Tests     []struct {
			Interval string `yaml:"interval"`
			Casos    []struct {
				Alerta string `yaml:"alertname"`
				Espera []any  `yaml:"exp_alerts"`
			} `yaml:"alert_rule_test"`
		} `yaml:"tests"`
	}
	if err := yaml.Unmarshal(crudo, &prueba); err != nil {
		t.Fatalf("%s no es YAML válido: %v", ruta, err)
	}
	if strings.Join(prueba.RuleFiles, ",") != archivoDeAlertasDelTablero {
		t.Errorf("la prueba de promtool carga %v y tiene que cargar SÓLO %s, al lado suyo: así se "+
			"copia junto a él en rules/ y se corre ahí", prueba.RuleFiles, archivoDeAlertasDelTablero)
	}
	dispara, calla := map[string]bool{}, map[string]bool{}
	for _, g := range prueba.Tests {
		for _, c := range g.Casos {
			if len(c.Espera) > 0 {
				dispara[c.Alerta] = true
			} else {
				calla[c.Alerta] = true
			}
		}
	}
	for nombre := range alertas {
		if !dispara[nombre] {
			t.Errorf("la prueba de promtool no tiene ningún caso donde %s DISPARE: una regla que no "+
				"puede dispararse nunca pasa todos los casos negativos en verde (es el defecto que el "+
				"plan original tenía en dos de las tres)", nombre)
		}
		if !calla[nombre] {
			t.Errorf("la prueba de promtool no tiene ningún caso donde %s se CALLE: una regla que "+
				"dispara siempre pasa todos los positivos en verde", nombre)
		}
	}
	for nombre := range dispara {
		if _, existe := alertas[nombre]; !existe {
			t.Errorf("la prueba de promtool espera %s, que ya no está en %s: ese caso prueba un "+
				"nombre que no existe", nombre, archivoDeAlertasDelTablero)
		}
	}
}
