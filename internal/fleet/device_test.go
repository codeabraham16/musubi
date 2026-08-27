package fleet

import (
	"errors"
	"testing"
	"time"
)

// A4 — la matriz del tier es el contrato, y las AUSENCIAS son lo que se prueba.
// Sabotaje que lo hace fallar: agregar CapScreen a TierProtocolo en capsPorTier.
func TestTierNoAdmiteLoQueSuHardwareNoTiene(t *testing.T) {
	casos := []struct {
		tier   Tier
		cap    Cap
		admite bool
		porque string
	}{
		{TierAgente, CapScreen, true, "un host con agente puede capturar pantalla"},
		{TierAgente, CapExec, true, "un host con agente puede ejecutar"},
		{TierProtocolo, CapMetrics, true, "SNMP/SSH entregan métricas"},
		{TierProtocolo, CapExec, true, "SSH ejecuta"},
		{TierProtocolo, CapScreen, false, "un router no tiene framebuffer"},
		{TierMovil, CapScreen, true, "un móvil sí tiene pantalla"},
		{TierMovil, CapExec, false, "iOS no da shell y en Android depende de ADB"},
	}
	for _, c := range casos {
		if got := TierAdmite(c.tier, c.cap); got != c.admite {
			t.Errorf("TierAdmite(%s, %s) = %v, esperaba %v — %s", c.tier, c.cap, got, c.admite, c.porque)
		}
	}
}

// A4 — conceder fuera de la matriz falla EN EL ALTA, no en el uso.
// Sabotaje: quitar el bucle de capacidades de ValidarAlta → el alta pasa y el bug aparece
// recién cuando alguien pide la pantalla de un router.
func TestAltaRechazaCapacidadQueElTierNoPuedeCumplir(t *testing.T) {
	d := Device{Name: "switch-sala", ProjectID: "infra", Tier: TierProtocolo, Caps: []Cap{CapMetrics, CapScreen}}
	err := ValidarAlta(d)
	if !errors.Is(err, ErrCapFueraDeTier) {
		t.Fatalf("esperaba ErrCapFueraDeTier, obtuve %v", err)
	}
	// El mensaje tiene que decir qué SÍ admite: un error que no ofrece la salida obliga a leer el código.
	if got := err.Error(); got == "" || !contiene(got, "metrics,exec") {
		t.Errorf("el error no dice qué admite el tier: %q", got)
	}
}

// A5 — el default es NINGUNA capacidad. Es la valla contra el puente de privilegio:
// administrar la memoria no puede otorgar control sobre las máquinas.
// Sabotaje: hacer que Permite devuelva true cuando Caps está vacío.
func TestDeviceCeroNoPermiteNada(t *testing.T) {
	var cero Device
	for _, c := range []Cap{CapMetrics, CapExec, CapScreen} {
		if cero.Permite(c) {
			t.Errorf("un Device cero permitió %q — el default tiene que ser fail-closed", c)
		}
	}
}

// A5 — un dispositivo revocado no permite nada, aunque la fila conserve las capacidades.
// Sabotaje: quitar la guarda `if d.Revoked` de Permite.
func TestDeviceRevocadoNoPermiteNadaAunqueConserveSusCaps(t *testing.T) {
	d := Device{Name: "pc-gio", ProjectID: "casa", Tier: TierAgente, Caps: []Cap{CapMetrics, CapExec, CapScreen}, Revoked: true}
	for _, c := range []Cap{CapMetrics, CapExec, CapScreen} {
		if d.Permite(c) {
			t.Errorf("un device revocado permitió %q: revocar tiene que cortar sin vaciar la lista", c)
		}
	}
}

// A4 (cinturón y tirantes) — una fila escrita a mano o por un binario viejo no elude la matriz.
// Sabotaje: que Permite devuelva true sin consultar TierAdmite.
func TestFilaConCapImposibleNoLaHonraIgual(t *testing.T) {
	// Fila corrupta: Tier B con `screen` concedido (ValidarAlta lo habría rechazado).
	d := Device{Name: "nas", ProjectID: "infra", Tier: TierProtocolo, Caps: []Cap{CapScreen}}
	if d.Permite(CapScreen) {
		t.Error("honró `screen` en un Tier B: la matriz tiene que valer también en lectura")
	}
}

// A6 — sin project_id no hay alta. Es el bug ya medido con las observaciones
// (2 filas sin atribuir visibles desde los 3 proyectos).
// Sabotaje: quitar el chequeo de ProjectID de ValidarAlta.
func TestAltaSinProyectoFalla(t *testing.T) {
	d := Device{Name: "huerfano", Tier: TierAgente, Caps: []Cap{CapMetrics}}
	if err := ValidarAlta(d); !errors.Is(err, ErrSinProyecto) {
		t.Fatalf("esperaba ErrSinProyecto, obtuve %v", err)
	}
}

func TestAltaSinNombreFalla(t *testing.T) {
	d := Device{ProjectID: "casa", Tier: TierAgente}
	if err := ValidarAlta(d); !errors.Is(err, ErrSinNombre) {
		t.Fatalf("esperaba ErrSinNombre, obtuve %v", err)
	}
}

func TestAltaConTierInventadoFalla(t *testing.T) {
	d := Device{Name: "x", ProjectID: "casa", Tier: Tier("Z")}
	if err := ValidarAlta(d); !errors.Is(err, ErrTierDesconocido) {
		t.Fatalf("esperaba ErrTierDesconocido, obtuve %v", err)
	}
}

// A8 — «en línea» se deriva, y el umbral lo elige quien pregunta.
// Sabotaje: devolver true con LastSeen cero → todo device recién dado de alta figura vivo.
func TestEnLineaSeDerivaDelUltimoLatido(t *testing.T) {
	ahora := time.Date(2026, 8, 26, 12, 0, 0, 0, time.UTC)
	casos := []struct {
		nombre   string
		lastSeen time.Time
		umbral   time.Duration
		quiero   bool
	}{
		{"latió recién", ahora.Add(-10 * time.Second), time.Minute, true},
		{"latió justo en el umbral", ahora.Add(-time.Minute), time.Minute, true},
		{"latió hace demasiado", ahora.Add(-5 * time.Minute), time.Minute, false},
		{"nunca latió", time.Time{}, time.Minute, false},
		{"umbral inválido", ahora, 0, false},
		// El mismo device, dos umbrales: el panel lo ve caído y la alerta todavía no.
		{"umbral flojo lo da por vivo", ahora.Add(-4 * time.Minute), 10 * time.Minute, true},
	}
	for _, c := range casos {
		d := Device{Name: "pc", ProjectID: "casa", Tier: TierAgente, LastSeen: c.lastSeen}
		if got := d.EnLinea(ahora, c.umbral); got != c.quiero {
			t.Errorf("%s: EnLinea = %v, esperaba %v", c.nombre, got, c.quiero)
		}
	}
}

// A8/A9 — un device revocado nunca está en línea, aunque su último latido sea de hace un segundo.
func TestDeviceRevocadoNuncaEstaEnLinea(t *testing.T) {
	ahora := time.Now()
	d := Device{Name: "pc", ProjectID: "casa", Tier: TierAgente, LastSeen: ahora, Revoked: true}
	if d.EnLinea(ahora, time.Minute) {
		t.Error("un device revocado figuró en línea")
	}
}

func TestNormalizarTierEsToleranteEnLaEntradaYEstrictoAdentro(t *testing.T) {
	for _, in := range []string{"A", "a", " agente ", "AGENT", "nativo"} {
		if got, err := NormalizarTier(in); err != nil || got != TierAgente {
			t.Errorf("NormalizarTier(%q) = %q, %v — esperaba A", in, got, err)
		}
	}
	if _, err := NormalizarTier("servidor"); !errors.Is(err, ErrTierDesconocido) {
		t.Errorf("esperaba ErrTierDesconocido, obtuve %v", err)
	}
}

// El orden canónico (metrics < exec < screen) es también el orden de poder: la fila guardada no
// puede depender de cómo escribió la lista el cliente.
func TestNormalizarCapsDeduplicaYOrdenaPorPoder(t *testing.T) {
	got, err := NormalizarCaps([]string{"screen", "metrics", "SCREEN", " exec ", ""})
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	quiero := []Cap{CapMetrics, CapExec, CapScreen}
	if len(got) != len(quiero) {
		t.Fatalf("obtuve %v, esperaba %v", got, quiero)
	}
	for i := range quiero {
		if got[i] != quiero[i] {
			t.Fatalf("obtuve %v, esperaba %v (orden canónico)", got, quiero)
		}
	}
	if _, err := NormalizarCaps([]string{"root"}); !errors.Is(err, ErrCapDesconocida) {
		t.Errorf("esperaba ErrCapDesconocida, obtuve %v", err)
	}
}

// Ida y vuelta por la columna CSV. Una capacidad desconocida en la fila se DESCARTA (fail-closed)
// en vez de romper el listado: no poder listar la flota por un campo ilegible sería peor.
// Sabotaje: hacer que CapsDesdeTexto devuelva la cap desconocida tal cual.
func TestCapsIdaYVueltaYBasuraSeDescarta(t *testing.T) {
	cs := []Cap{CapMetrics, CapExec}
	if got := CapsDesdeTexto(CapsComoTexto(cs)); len(got) != 2 || got[0] != CapMetrics || got[1] != CapExec {
		t.Fatalf("ida y vuelta perdió capacidades: %v", got)
	}
	got := CapsDesdeTexto("metrics, root ,,screen")
	if len(got) != 2 || got[0] != CapMetrics || got[1] != CapScreen {
		t.Fatalf("esperaba [metrics screen] descartando `root`, obtuve %v", got)
	}
	if CapsComoTexto(nil) != "" {
		t.Error("una lista vacía tiene que serializar a cadena vacía")
	}
}

// La matriz no se puede mutar desde afuera.
// Sabotaje: devolver el slice interno en vez de una copia.
func TestCapsDelTierDevuelveCopia(t *testing.T) {
	primera := CapsDelTier(TierAgente)
	primera[0] = CapExec
	if segunda := CapsDelTier(TierAgente); segunda[0] != CapMetrics {
		t.Error("mutar el resultado de CapsDelTier corrompió la matriz global")
	}
}

func contiene(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// A8 — la guarda `LastSeen.IsZero()` fijada por el ÚNICO caso donde es load-bearing.
//
// Medido al escribir esto: `time.Duration` satura en ~292 años, así que con un reloj real un
// LastSeen cero ya da `false` por aritmética y la guarda no se nota. Con un reloj CERO —un
// llamador que no inicializó su fuente de tiempo— `cero.Sub(cero)` es 0 y entra en cualquier
// umbral. Sin la guarda, un dispositivo que nunca latió se reporta EN LÍNEA.
//
// Sabotaje que lo hace fallar: quitar `d.LastSeen.IsZero()` de la condición de EnLinea.
func TestEnLineaConRelojCeroNoInventaVida(t *testing.T) {
	d := Device{Name: "recien-dado-de-alta", ProjectID: "casa", Tier: TierAgente} // LastSeen cero
	if d.EnLinea(time.Time{}, time.Minute) {
		t.Error("con reloj cero y sin latidos, el device figuró en línea: la guarda IsZero no está")
	}
	// Y con reloj real sigue caído (esto lo sostiene la saturación, no la guarda — se deja
	// explícito para que nadie 'simplifique' la guarda creyendo que este caso la cubre).
	if d.EnLinea(time.Now(), time.Minute) {
		t.Error("con reloj real y sin latidos, el device figuró en línea")
	}
}
