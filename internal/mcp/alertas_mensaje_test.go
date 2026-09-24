package mcp

// alertas_mensaje_test.go RENDERIZA la plantilla de Telegram con datos de verdad.
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// POR QUÉ RENDERIZAR Y NO BUSCAR TEXTO
//
// El resto de las guardas de despliegue leen los archivos y buscan cadenas. Sobre una plantilla
// eso no alcanza: la versión anterior contenía todas las palabras correctas —severidad, máquina,
// resumen, runbook— y aun así producía un mensaje ilegible, porque el problema no estaba en lo
// que decía sino en lo que NO distinguía.
//
// La falla concreta, que vivió meses sin que nadie la viera: `telegram_configs` manda las
// resoluciones por default, y la plantilla no miraba `.Status`. Un «se arregló» llegaba BYTE POR
// BYTE IDÉNTICO a un «se rompió». Nadie lo notó porque nadie había renderizado nunca la
// plantilla — se la leía, que es otra cosa.
//
// Esta prueba usa la MISMA estructura de datos que le pasa Alertmanager, incluido el
// `CommonLabels` que él calcula (las etiquetas que TODAS las alertas del grupo comparten).

import (
	"strings"
	"testing"
	"text/template"
	"time"

	"gopkg.in/yaml.v3"
)

type alertaDePrueba struct {
	Status      string
	Labels      map[string]string
	Annotations map[string]string
	StartsAt    time.Time
	EndsAt      time.Time
}

type datosDeAlerta struct {
	Status       string
	Alerts       []alertaDePrueba
	GroupLabels  map[string]string
	CommonLabels map[string]string
}

// comunes replica lo que Alertmanager pone en CommonLabels: la intersección de las etiquetas.
// Se calcula acá para que la prueba no mienta sobre lo que llega.
func comunes(as []alertaDePrueba) map[string]string {
	out := map[string]string{}
	if len(as) == 0 {
		return out
	}
	for k, v := range as[0].Labels {
		out[k] = v
	}
	for _, a := range as[1:] {
		for k, v := range out {
			if a.Labels[k] != v {
				delete(out, k)
			}
		}
	}
	return out
}

func plantillaDeTelegram(t *testing.T) *template.Template {
	t.Helper()
	b, err := leerArchivoDeDespliegue("../../deploy/prometheus/alertmanager.yml")
	if err != nil {
		t.Fatalf("falta alertmanager.yml: %v", err)
	}
	var cfg struct {
		Receivers []struct {
			Name     string `yaml:"name"`
			Telegram []struct {
				Message   string `yaml:"message"`
				ParseMode string `yaml:"parse_mode"`
			} `yaml:"telegram_configs"`
		} `yaml:"receivers"`
	}
	if err := yaml.Unmarshal(b, &cfg); err != nil {
		t.Fatalf("alertmanager.yml no parsea: %v", err)
	}
	for _, r := range cfg.Receivers {
		if r.Name != "default" || len(r.Telegram) == 0 {
			continue
		}
		// TEXTO PLANO, y no es cosmético: con parse_mode HTML un `&` o un `<` en cualquier
		// resumen rompe el parseo y Telegram RECHAZA el mensaje entero. La alerta no llega.
		if pm := strings.TrimSpace(r.Telegram[0].ParseMode); pm != "" {
			t.Errorf("parse_mode = %q: un caracter especial en un resumen haría que Telegram "+
				"rechace el mensaje y la alerta NO LLEGUE", pm)
		}
		tpl, err := template.New("m").Parse(r.Telegram[0].Message)
		if err != nil {
			t.Fatalf("la plantilla no compila: %v", err)
		}
		return tpl
	}
	t.Fatal("no hay receptor `default` con telegram_configs")
	return nil
}

func rendir(t *testing.T, tpl *template.Template, d datosDeAlerta) string {
	t.Helper()
	d.CommonLabels = comunes(d.Alerts)
	var b strings.Builder
	if err := tpl.Execute(&b, d); err != nil {
		t.Fatalf("la plantilla falló al renderizar: %v", err)
	}
	out := b.String()
	// `<no value>` es lo que imprime text/template cuando se pide un campo que no existe. Llega
	// al teléfono tal cual y no dice nada.
	if strings.Contains(out, "<no value>") {
		t.Errorf("el mensaje contiene `<no value>`:\n%s", out)
	}
	return out
}

func unaAlerta(st string, inicio time.Time, extra map[string]string) alertaDePrueba {
	lab := map[string]string{"alertname": "DiscoPorLlenarse", "device": "davantis-1", "severity": "warning"}
	for k, v := range extra {
		lab[k] = v
	}
	a := alertaDePrueba{Status: st, Labels: lab, StartsAt: inicio,
		Annotations: map[string]string{"summary": "davantis-1 tiene 6.8% de disco libre.", "runbook": "deploy/RUNBOOK.md#discoporllenarse"}}
	if st == "resolved" {
		a.EndsAt = inicio.Add(52 * time.Minute)
	}
	return a
}

// UN «SE ARREGLÓ» NO PUEDE LLEGAR IGUAL QUE UN «SE ROMPIÓ».
//
// Es la falla que hacía ilegible el canal entero: cada mensaje obligaba a ir a mirar, aunque
// fuera una buena noticia.
//
// Sabotaje: sacar el `{{ if eq .Status "firing" }}` de la primera línea → falla acá.
// arnes: archivo="deploy/prometheus/alertmanager.yml"
// arnes: de="{{ if eq .Status \"firing\" }}🔴 EMPIEZA{{ else }}✅ SE RESOLVIÓ{{ end }}"
// arnes: a="🔴 EMPIEZA"
func TestUnaAlertaResueltaNoSeLeeIgualQueUnaQueEmpieza(t *testing.T) {
	tpl := plantillaDeTelegram(t)
	inicio := time.Date(2026, 9, 2, 3, 12, 0, 0, time.UTC)

	empieza := rendir(t, tpl, datosDeAlerta{Status: "firing",
		GroupLabels: map[string]string{"alertname": "DiscoPorLlenarse", "device": "davantis-1"},
		Alerts:      []alertaDePrueba{unaAlerta("firing", inicio, nil)}})
	resuelta := rendir(t, tpl, datosDeAlerta{Status: "resolved",
		GroupLabels: map[string]string{"alertname": "DiscoPorLlenarse", "device": "davantis-1"},
		Alerts:      []alertaDePrueba{unaAlerta("resolved", inicio, nil)}})

	if empieza == resuelta {
		t.Fatalf("el aviso de que algo SE RESOLVIÓ llega idéntico al de que EMPEZÓ:\n%s", empieza)
	}
	// Y LA DIFERENCIA TIENE QUE ESTAR ARRIBA. Un mensaje que sólo se distingue en la última
	// línea obliga a leerlo entero para saber si es una buena o una mala noticia.
	primeraE := strings.SplitN(empieza, "\n", 2)[0]
	primeraR := strings.SplitN(resuelta, "\n", 2)[0]
	if primeraE == primeraR {
		t.Errorf("la primera línea no distingue empieza de resuelta:\n  %q\n  %q", primeraE, primeraR)
	}
}

// UN GRUPO NO REPITE LA CABECERA POR CADA ALERTA.
//
// El agrupado junta por (alerta × máquina). La plantilla vieja hacía `range .Alerts` imprimiendo
// la cabecera entera cada vez: cuatro servicios caídos eran cuatro bloques donde sólo variaba una
// línea, y dieciséis eran un muro que nadie lee.
//
// EL FIXTURE LE PASABA LA RESPUESTA A LA GUARDA (medido el 2026-09-19). El `summary` de cada alerta
// decía «El servicio sppsvc de gio no está corriendo», o sea que repetía el nombre del servicio que
// la aserción de abajo busca. Con eso, cambiar la viñeta de `{{ .Labels.service }}` a
// `{{ .Annotations.summary }}` —que es exactamente el muro que esta prueba existe para prohibir—
// dejaba la guarda en VERDE: el nombre aparecía igual, por el otro camino. La guarda no distinguía
// «imprimió la ETIQUETA» de «imprimió la ANOTACIÓN», que es la única diferencia que le importa.
//
// Ahora el `summary` no nombra el servicio. Es un fixture saneado aguas arriba al revés: no hay que
// sembrar el dato en dos lugares y esperar que la aserción adivine cuál leyó.
//
// Y SE EJERCITA LA RAMA DE `policy`, que no la miraba nadie. La viñeta tiene tres caminos —
// `.Labels.service`, `.Labels.policy`, y el `summary` como último recurso— y sólo el primero estaba
// cubierto. `musubi_fleet_policy_actions_total` no lleva `device` ni `project` a propósito, así que
// `policy` es la ÚNICA etiqueta que distingue dos alertas de política en un grupo.
//
// Sabotaje que la hace fallar: que la viñeta imprima el `summary` en vez de la etiqueta que
// distingue. El otro, que ya estaba declarado acá, sigue valiendo y no se mecaniza porque es
// estructural: volver a envolver todo el cuerpo en un `range .Alerts`.
// arnes: archivo="deploy/prometheus/alertmanager.yml"
// arnes: de="{{ if .Labels.service }}{{ .Labels.service }}{{ else if .Labels.policy }}{{ .Labels.policy }}{{ else }}{{ .Annotations.summary }}{{ end }}"
// arnes: a="{{ .Annotations.summary }}"
func TestUnGrupoNoRepiteLaCabeceraPorCadaAlerta(t *testing.T) {
	tpl := plantillaDeTelegram(t)
	inicio := time.Date(2026, 9, 2, 3, 12, 0, 0, time.UTC)

	for _, caso := range []struct {
		alerta   string
		etiqueta string
		valores  []string
		runbook  string
		resumen  string
	}{
		{"ServicioCaido", "service",
			[]string{"sppsvc", "MapsBroker", "edgeupdate", "GoogleUpdaterService152.0.7933.0"},
			"deploy/RUNBOOK.md#serviciocaido",
			// NO nombra el servicio: si lo nombrara, la aserción de abajo no podría distinguir
			// la etiqueta de la anotación, que es justo lo que se está midiendo.
			"Un servicio de gio no está corriendo."},
		{"PoliticaFrenadaPorConsentimiento", "policy",
			[]string{"vaciar-journal", "reiniciar-relay"},
			"deploy/RUNBOOK.md#politicafrenadaporconsentimiento",
			"Una política no actúa sobre alguna máquina porque su dueño no lo permite."},
	} {
		t.Run(caso.alerta, func(t *testing.T) {
			var as []alertaDePrueba
			for _, v := range caso.valores {
				as = append(as, alertaDePrueba{Status: "firing", StartsAt: inicio,
					Labels:      map[string]string{"alertname": caso.alerta, "device": "gio", caso.etiqueta: v, "severity": "warning"},
					Annotations: map[string]string{"summary": caso.resumen, "runbook": caso.runbook}})
			}
			out := rendir(t, tpl, datosDeAlerta{Status: "firing",
				GroupLabels: map[string]string{"alertname": caso.alerta, "device": "gio"}, Alerts: as})

			if n := strings.Count(out, caso.alerta); n != 1 {
				t.Errorf("el nombre de la alerta aparece %d veces en un grupo de %d: la cabecera se está repitiendo:\n%s",
					n, len(as), out)
			}
			// PERO CADA CASO TIENE QUE VERSE. Un mensaje que agrupa y no dice QUÉ agrupó es peor
			// que la repetición: obliga a ir al panel para saber de qué está hablando. Y el
			// `summary` del fixture NO lleva estos valores, así que esto sólo puede pasar si la
			// plantilla imprimió la etiqueta.
			for _, v := range caso.valores {
				if !strings.Contains(out, v) {
					t.Errorf("el grupo no nombra a %q: la viñeta dejó de imprimir `.Labels.%s` y hay que\n"+
						"ir al panel para saber a qué se refiere. Dos alertas de esta familia en la misma\n"+
						"máquina se leen idénticas.\n%s", v, caso.etiqueta, out)
				}
			}
		})
	}
}

// UN GRUPO CON SEVERIDADES DISTINTAS NO IMPRIME BASURA.
//
// Si las alertas de un grupo no comparten `severity`, Alertmanager NO la pone en CommonLabels y
// la plantilla imprime `<no value>` — que llega al teléfono tal cual. Se descubrió renderizando
// la plantilla con datos de verdad; leyéndola no se ve.
//
// Sabotaje: sacar el `{{ if .CommonLabels.severity }}` → falla en la comprobación de `<no value>`
// que hace `rendir`.
// arnes: archivo="deploy/prometheus/alertmanager.yml"
// arnes: de="{{ if .CommonLabels.severity }}{{ .CommonLabels.severity }}{{ else }}severidad mixta{{ end }}"
// arnes: a="{{ .CommonLabels.severity }}"
func TestUnGrupoConSeveridadesDistintasNoImprimeBasura(t *testing.T) {
	tpl := plantillaDeTelegram(t)
	inicio := time.Date(2026, 9, 2, 3, 12, 0, 0, time.UTC)

	as := []alertaDePrueba{
		unaAlerta("firing", inicio, map[string]string{"severity": "warning", "service": "uno"}),
		unaAlerta("firing", inicio, map[string]string{"severity": "critical", "service": "dos"}),
	}
	out := rendir(t, tpl, datosDeAlerta{Status: "firing",
		GroupLabels: map[string]string{"alertname": "DiscoPorLlenarse", "device": "davantis-1"}, Alerts: as})
	if strings.TrimSpace(out) == "" {
		t.Error("el mensaje salió vacío con severidades mixtas")
	}
}
