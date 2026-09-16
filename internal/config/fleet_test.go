package config

// Pruebas de la configuración de flota (S10). Los defaults importan más de lo habitual acá: de
// `probe_minutes` se DERIVA el umbral de «en línea» de las máquinas sin agente, así que un cero
// mal interpretado no da un sondeo lento — da una flota entera figurando caída.

import (
	"testing"
	"time"
)

// El default y el apagado explícito son cosas distintas, y confundirlas rompe en direcciones
// opuestas: 0 (no lo escribí) tiene que dar el default; negativo (lo apagué) tiene que apagar.
//
// Sabotaje que la hace fallar: tratar cualquier valor <= 0 como "desactivado", que es el atajo
// obvio y deja al que no configuró nada sin sondeo — y por lo tanto con toda la flota sin agente
// figurando caída para siempre.
func TestElIntervaloDeSondeoDistingueElDefaultDelApagado(t *testing.T) {
	casos := []struct {
		nombre  string
		cfg     FleetConfig
		esperar time.Duration
	}{
		{"sin escribir nada ⇒ default", FleetConfig{}, 5 * time.Minute},
		{"explícito", FleetConfig{ProbeMinutes: 2}, 2 * time.Minute},
		{"fracción de minuto", FleetConfig{ProbeMinutes: 0.5}, 30 * time.Second},
		{"negativo ⇒ apagado", FleetConfig{ProbeMinutes: -1}, 0},
	}
	for _, c := range casos {
		if got := c.cfg.EffectiveProbeInterval(); got != c.esperar {
			t.Errorf("%s: intervalo = %v, esperaba %v", c.nombre, got, c.esperar)
		}
	}
}

// Lo mismo para la retención de salidas: no escribir nada tiene que dar una retención real, no
// "guardar para siempre". Una tabla de salidas de comandos sin techo es el problema que la
// bitácora vino a resolver.
func TestLaRetencionDeSalidasDistingueElDefaultDelApagado(t *testing.T) {
	casos := []struct {
		nombre  string
		cfg     FleetConfig
		esperar int
	}{
		{"sin escribir nada ⇒ 30 días", FleetConfig{}, 30},
		{"explícito", FleetConfig{CommandOutputRetentionDays: 7}, 7},
		{"negativo ⇒ no caducan", FleetConfig{CommandOutputRetentionDays: -1}, 0},
	}
	for _, c := range casos {
		if got := c.cfg.EffectiveOutputRetentionDays(); got != c.esperar {
			t.Errorf("%s: retención = %d, esperaba %d", c.nombre, got, c.esperar)
		}
	}
}

// Un Config sin sección `fleet:` tiene que dar exactamente los mismos defaults que uno con la
// sección vacía: es lo que garantiza que estrenar S10 no cambie el comportamiento de nadie que no
// haya tocado su configuración.
func TestUnConfigSinSeccionDeFlotaUsaLosMismosDefaults(t *testing.T) {
	var vacio Config
	if vacio.Fleet.EffectiveProbeInterval() != (FleetConfig{}).EffectiveProbeInterval() {
		t.Error("un Config sin sección fleet: no coincide con una sección vacía")
	}
	if len(vacio.Fleet.Policies) != 0 {
		t.Error("las políticas tienen que nacer apagadas")
	}
}

// EL TECHO DE SERVICIOS DEL EXPORT: 0 ⇒ default, negativo ⇒ SIN TECHO.
//
// Los dos extremos importan y no son simétricos: el 0 del campo es «no lo escribí» —un YAML sin
// la clave— y tiene que caer en el default, mientras que el negativo es «lo escribí a propósito
// para desactivarlo». Si el 0 significara «sin techo», cualquier config que no nombre la clave
// exportaría sin ninguna cota de cardinalidad, que es justo lo que este número existe para evitar.
func TestElTechoDeServiciosDelExportDistingueElDefaultDelApagado(t *testing.T) {
	casos := []struct {
		nombre  string
		cfg     FleetConfig
		esperar int
	}{
		{"sin la clave ⇒ default", FleetConfig{}, 2000},
		{"un número ⇒ ese número", FleetConfig{ServicesPerProjectExport: 300}, 300},
		{"negativo ⇒ sin techo (0)", FleetConfig{ServicesPerProjectExport: -1}, 0},
	}
	for _, c := range casos {
		if got := c.cfg.EffectiveServicesPerProjectExport(); got != c.esperar {
			t.Errorf("%s: techo = %d, esperaba %d", c.nombre, got, c.esperar)
		}
	}
}

// EL TECHO DE APROBACIONES NO TIENE APAGADO, Y ESA ASIMETRÍA CON SU HERMANO ES EL PUNTO (A124).
//
// `EffectiveServicesPerProjectExport` lee un negativo como «sin techo» y devuelve 0. Copiar esa
// convención acá habría sido un defecto silencioso: `AprobacionesPendientes`
// (internal/memory/aprobaciones.go) hace `if tope <= 0 { tope = 50 }`, así que un 0 que viajara
// hasta la consulta NO desactivaría el techo — lo APRETARÍA de 200 a 50. Alguien que escribiera
// `approvals_per_project_export: -1` por analogía con la perilla de al lado habría perdido tres
// cuartas partes de la página creyendo que la sacaba, y sin una sola línea que lo dijera.
//
// Por eso esta función CLAMPEA en vez de propagar, y la guarda mide justamente eso: que no exista
// ninguna entrada que la haga devolver un valor que el almacén vaya a reinterpretar. No alcanza
// con probar −1: lo que hay que sostener es la propiedad «nunca <= 0», que es la que hace
// irrepresentable el camino malo.
//
// Sabotaje que la hace fallar: dejar pasar los negativos (`== 0` en vez de `<= 0`).
// arnes: archivo="internal/config/config.go"
// arnes: de="if f.ApprovalsPerProjectExport <= 0 {"
// arnes: a="if f.ApprovalsPerProjectExport == 0 {"
func TestElTechoDeAprobacionesNuncaLeEntregaAlAlmacenUnValorQueVaAReinterpretar(t *testing.T) {
	casos := []struct {
		nombre  string
		cfg     FleetConfig
		esperar int
	}{
		{"sin la clave ⇒ default", FleetConfig{}, 200},
		{"un número ⇒ ese número", FleetConfig{ApprovalsPerProjectExport: 500}, 500},
		{"cero explícito ⇒ default", FleetConfig{ApprovalsPerProjectExport: 0}, 200},
		{"negativo ⇒ default, NO «sin techo»", FleetConfig{ApprovalsPerProjectExport: -1}, 200},
	}
	for _, c := range casos {
		if got := c.cfg.EffectiveApprovalsPerProjectExport(); got != c.esperar {
			t.Errorf("%s: techo = %d, esperaba %d", c.nombre, got, c.esperar)
		}
	}
	// LA PROPIEDAD, no los tres casos de arriba: ningún valor de entrada puede producir un techo
	// <= 0, porque ahí el almacén dejaría de leer el número como un techo.
	for _, v := range []int{-1000, -200, -50, -1, 0, 1, 50, 200, 5000} {
		if got := (FleetConfig{ApprovalsPerProjectExport: v}).EffectiveApprovalsPerProjectExport(); got <= 0 {
			t.Errorf("con `approvals_per_project_export: %d` el techo efectivo es %d; el almacén leería eso como 50 y apretaría el techo en vez de sacarlo", v, got)
		}
	}
}
