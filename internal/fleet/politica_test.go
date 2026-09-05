package fleet

// Pruebas del dominio puro de S10: la allowlist y la decisión de una política.
// Cada prueba dice qué sabotaje la hace fallar; las que no lo dicen no valen nada.

import (
	"testing"
	"time"
)

// I10 — EL MATCH ES SOBRE argv[0] EXACTO, y la razón es un bypass concreto.
//
// Comparar por basename es la implementación "obvia" y es la que deja pasar /tmp/evil/systemctl
// contra una entrada `systemctl`. Cualquiera que pueda escribir en un directorio de la máquina
// —o sea, cualquiera que tenga un shell— se salta la allowlist entera plantando un binario con
// el nombre correcto.
//
// Sabotaje que la hace fallar: comparar filepath.Base(limpio[0]) contra filepath.Base(p).
func TestLaAllowlistNoSeSaltaConUnaRutaQueTermineIgual(t *testing.T) {
	permitidos := []string{"systemctl", "journalctl"}

	if !PermiteArgv(permitidos, []string{"systemctl", "restart", "nginx"}) {
		t.Error("un comando permitido tal cual debería pasar")
	}
	// El bypass: mismo basename, otro binario.
	for _, argv := range [][]string{
		{"/tmp/evil/systemctl", "restart", "nginx"},
		{"./systemctl"},
		{"../../tmp/systemctl"},
	} {
		if PermiteArgv(permitidos, argv) {
			t.Errorf("BYPASS: %v pasó una allowlist que sólo nombra %v — el basename no puede ser el criterio", argv, permitidos)
		}
	}
}

// I9 — UNA LISTA VACÍA NO PERMITE NADA.
//
// El bug clásico de las allowlists es `if len(lista) == 0 { return true }`: parece defensivo
// («no configuró nada, no restrinjo») y es exactamente lo contrario de lo que la palabra
// allowlist significa. Acá es un caso con nombre propio para que nadie lo "arregle".
//
// Sabotaje que la hace fallar: devolver true cuando len(permitidos) == 0.
func TestUnaAllowlistVaciaNoPermiteNada(t *testing.T) {
	if PermiteArgv(nil, []string{"uptime"}) {
		t.Error("una allowlist nil no puede permitir un comando: «sin restricción» se expresa NO teniendo lista, y eso lo decide quien llama")
	}
	if PermiteArgv([]string{}, []string{"uptime"}) {
		t.Error("una allowlist vacía permitió un comando: es el `len == 0 ⇒ pasa todo` que la palabra allowlist prohíbe")
	}
}

// I10b — un intérprete en la allowlist ES la allowlist entera.
func TestSeReconoceAlInterpreteQueAnulaLaAllowlist(t *testing.T) {
	for _, c := range []string{"sh", "bash", "/bin/bash", "python3", "sudo", "xargs", "PowerShell.exe"} {
		if !EsInterprete(c) {
			t.Errorf("%q debería avisarse: puede lanzar cualquier otro comando", c)
		}
	}
	for _, c := range []string{"systemctl", "journalctl", "df", "uptime", ""} {
		if EsInterprete(c) {
			t.Errorf("%q no es un intérprete; avisar de más entrena a ignorar el aviso", c)
		}
	}
}

// ── Las políticas ───────────────────────────────────────────────────────────────────────────

// I13 (la mitad del dominio) — UN DATO AUSENTE NO DISPARA, Y NO ES UN CERO.
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// LA PRIMERA VERSIÓN DE ESTA PRUEBA NO SERVÍA, Y VALE LA PENA QUE QUEDE ESCRITO POR QUÉ.
//
// Probaba `carga_por_core > 2` y `temp_c > 85` contra una muestra sin esos datos. Con el sabotaje
// puesto (leer un nil como 0) la prueba PASABA IGUAL: 0 > 2 es falso, así que no disparaba, así
// que "no disparó" no decía nada sobre si el nil se había leído como cero o se había respetado
// como ausente. Estaba midiendo el lado inofensivo del error.
//
// El lado peligroso es la ÚNICA condición que dispara al BAJAR. `disco_libre_pct < 10` contra una
// muestra sin disco: si el ausente se lee como 0, 0 < 10 es VERDADERO y la política sale a correr
// un comando de limpieza en una máquina cuyo disco NUNCA SE MIDIÓ. Y esa muestra es alcanzable —
// Muestra.Valida() acepta disco en cero, que es justo lo que produce un OS sin colector de disco.
//
// La regla, otra vez: una guarda de defensa en profundidad casi nunca la fija la prueba del
// camino feliz. Hay que simular el error del que protege.
// ────────────────────────────────────────────────────────────────────────────────────────────
//
// Sabotaje que la hace fallar: en CondDiscoLibrePct, devolver 0 en vez de (0,false) cuando
// PctUsado da nil.
func TestUnDiscoQueNuncaSeMidioNoDisparaLaLimpieza(t *testing.T) {
	// Un OS sin colector de disco: los tres campos en cero. La muestra es VÁLIDA.
	sinDisco := &Muestra{Tomada: time.Now(), NumCPU: 4, MemTotal: 8 << 30, MemUsada: 1 << 30}
	if err := sinDisco.Valida(); err != nil {
		t.Fatalf("la muestra tiene que ser válida o la prueba no representa nada real: %v", err)
	}

	limpieza := Politica{Nombre: "limpiar", Principal: "auto", Cuando: CondDiscoLibrePct, Supera: 10,
		Sobre: []string{"*"}, Hacer: []string{"journalctl", "--vacuum-size=200M"}}
	if v, dispara := limpieza.Dispara(sinDisco); dispara {
		t.Errorf("DISPARÓ con el disco SIN MEDIR (valor=%v): un ausente se leyó como 0%% libre y la política saldría a borrar en una máquina de la que no sabemos nada", v)
	}

	// Control positivo: con el disco medido y realmente lleno, TIENE que disparar. Sin esto, la
	// prueba de arriba pasaría también con un Dispara() que devuelve false siempre.
	lleno := &Muestra{Tomada: time.Now(), DiscoTotal: 1000, DiscoUsado: 980, DiscoDisponible: 20}
	if _, dispara := limpieza.Dispara(lleno); !dispara {
		t.Error("con 2%% libre no disparó: la prueba negativa de arriba no probaría nada")
	}
}

// El mismo invariante en las condiciones que disparan al SUBIR. Acá el error es menos peligroso
// —un 0 no cruza un umbral alto— pero se fija igual, porque el día que alguien escriba
// `cpu_pct > 0` para "ver si está viva", cada máquina Windows de la flota daría positivo.
//
// Sabotaje que la hace fallar: reemplazar los punteros por su valor con default 0 Y bajar el
// umbral, o derivar carga_por_core sin chequear NumCPU > 0.
func TestUnaMetricaAusenteNoCuentaComoCeroMedido(t *testing.T) {
	// Una máquina Windows: sin load, sin sensor térmico, y la CPU todavía sin derivada.
	windows := &Muestra{
		Tomada: time.Now(), NumCPU: 8,
		MemTotal: 16 << 30, MemUsada: 2 << 30,
		DiscoTotal: 500 << 30, DiscoUsado: 100 << 30, DiscoDisponible: 380 << 30,
		Load5: nil, TempC: nil, CPUPct: nil,
	}
	// Umbral 0: «cualquier valor medido». Un ausente leído como 0 no lo cruza por poco, así que
	// se comprueba el CANAL —que Dispara diga que no hay dato— y no sólo el booleano.
	for _, cond := range []Condicion{CondCargaPorCore, CondTempC, CondCPUPct} {
		p := Politica{Nombre: "x", Principal: "y", Cuando: cond, Supera: -1, Sobre: []string{"*"}, Hacer: []string{"echo"}}
		if v, dispara := p.Dispara(windows); dispara {
			t.Errorf("%s disparó contra un umbral de -1 con el dato AUSENTE (valor=%v): un nil se está leyendo como un número", cond, v)
		}
	}
	// Y una muestra que SÍ tiene load: la misma condición tiene que disparar.
	l := 4.0
	conLoad := *windows
	conLoad.Load5 = &l
	carga := Politica{Nombre: "x", Principal: "y", Cuando: CondCargaPorCore, Supera: 0.4, Sobre: []string{"*"}, Hacer: []string{"echo"}}
	if v, dispara := carga.Dispara(&conLoad); !dispara {
		t.Errorf("carga_por_core = %v (4/8) no disparó contra 0.4: el control positivo falla", v)
	}
	// Y ojo con el divisor: sin NumCPU no hay carga por core, aunque haya load.
	sinCPUs := conLoad
	sinCPUs.NumCPU = 0
	if _, dispara := carga.Dispara(&sinCPUs); dispara {
		t.Error("carga_por_core disparó con NumCPU=0: se estaría dividiendo por cero o tratando 0 cores como 1")
	}
}

// El disco es la única condición que dispara al BAJAR, y se mira lo ESCRIBIBLE.
//
// No es un capricho: la reserva de root (~5 %) no es ni usado ni disponible, así que un disco
// «al 92 % usado» puede tener 0 bytes para una aplicación. Una política de limpieza que mirara
// `disco_pct` esperaría al 95 % de USADO mientras el servicio ya está fallando por ENOSPC.
func TestElDiscoSeMiraPorLoQueQuedaEscribibleYDisparaAlBajar(t *testing.T) {
	// 92 % usado, pero sólo 3 % escribible: la diferencia es la reserva.
	m := &Muestra{Tomada: time.Now(), DiscoTotal: 1000, DiscoUsado: 920, DiscoDisponible: 30}

	libre := Politica{Nombre: "l", Principal: "p", Cuando: CondDiscoLibrePct, Supera: 10, Sobre: []string{"*"}, Hacer: []string{"echo"}}
	v, dispara := libre.Dispara(m)
	if !dispara {
		t.Errorf("disco_libre_pct=%v no disparó con umbral 10: la condición del disco tiene que disparar al BAJAR", v)
	}
	if v != 3 {
		t.Errorf("disco_libre_pct = %v; esperaba 3 (30/1000): se está midiendo otra cosa", v)
	}
	// La misma máquina, mirada por lo usado, con el umbral que "parece" equivalente.
	usado := Politica{Nombre: "u", Principal: "p", Cuando: CondDiscoPct, Supera: 95, Sobre: []string{"*"}, Hacer: []string{"echo"}}
	if _, dispara := usado.Dispara(m); dispara {
		t.Error("disco_pct 92% no debería cruzar un umbral de 95%: es justo la trampa que disco_libre_pct evita")
	}
}

// Validar es de ARRANQUE (I12): lo que está mal escrito no puede convertirse en una alarma
// silenciosa. Se prueba cada rechazo porque cada uno tapa un modo de fallo distinto.
func TestUnaPoliticaMalEscritaNoArranca(t *testing.T) {
	base := func() Politica {
		return Politica{Nombre: "n", Principal: "auto", Cuando: CondMemPct, Supera: 90,
			Sobre: []string{"*"}, Hacer: []string{"systemctl", "restart", "x"}, Cooldown: time.Hour}
	}
	if err := base().Validar(); err != nil {
		t.Fatalf("la política base debería ser válida, o el resto de la prueba no dice nada: %v", err)
	}
	casos := map[string]func(*Politica){
		"sin nombre":             func(p *Politica) { p.Nombre = "" },
		"sin principal":          func(p *Politica) { p.Principal = "" },
		"condición inventada":    func(p *Politica) { p.Cuando = "disko_pct" },
		"sin máquinas":           func(p *Politica) { p.Sobre = []string{"  "} },
		"sin comando":            func(p *Politica) { p.Hacer = nil },
		"operación interna":      func(p *Politica) { p.Hacer = []string{"musubi:pantalla", "x"} },
		"cooldown de un segundo": func(p *Politica) { p.Cooldown = time.Second },
	}
	for nombre, romper := range casos {
		p := base()
		romper(&p)
		if err := p.Validar(); err == nil {
			t.Errorf("%s: Validar() la aceptó. Una política que no significa lo que su autor cree es peor que ninguna", nombre)
		}
	}
}

// El escalamiento que S6 cerró para `exec` vale igual para una política: si `musubi:pantalla`
// se pudiera poner en `hacer`, quien edite la config del cerebro se acuñaría sesiones de pantalla
// sin tener nunca la capacidad `screen`.
func TestUnaPoliticaNoPuedeFabricarMensajesInternosDelCanal(t *testing.T) {
	p := Politica{Nombre: "n", Principal: "auto", Cuando: CondMemPct, Supera: 1,
		Sobre: []string{"*"}, Hacer: []string{"musubi:pantalla", "clave"}}
	if err := p.Validar(); err == nil {
		t.Fatal("una política pudo encolar una operación interna del canal: es la misma puerta lateral que S6 cerró para exec")
	}
}
