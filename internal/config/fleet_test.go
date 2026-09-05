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
