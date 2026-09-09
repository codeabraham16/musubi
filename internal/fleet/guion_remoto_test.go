package fleet

// guion_remoto_test.go ATA EL GUION AL PARSER, que es el contrato que no tenía dueño.
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// POR QUÉ ESTE ARCHIVO EXISTE
//
// `guionLecturaProc` (remoto.go) manda ocho secciones separadas por una marca, y
// `ParsearLecturaRemota` las lee POR POSICIÓN: tomar(0) es /proc/stat, tomar(6) es el conteo de
// CPUs, tomar(7) es el listado de /proc. Son dos listas ordenadas en dos funciones distintas y
// NADA las ataba: hasta este archivo, ninguna prueba del repo nombraba siquiera la constante del
// guion. Las pruebas del parser arman su propio texto sintético con separadores, así que
// verifican el parser CONTRA SÍ MISMO — el guion podía decir cualquier cosa.
//
// El propio encabezado de remoto.go advierte del peligro («insertar una sección en el MEDIO corre
// todos los índices y no rompe nada visible»), y esa advertencia era exactamente la parte sin
// prueba. Verificado: moviendo `ls -1 /proc` a la segunda sección y sin tocar una línea del
// parser, la suite entera seguía en verde mientras un Tier B real producía MemTotal=0,
// DiscoTotal=0, Load1=nil y NumCPU=27800 —la temperatura en miligrados leída como conteo de
// procesadores— con `Valida()` devolviendo nil, o sea guardado y dibujado como si fuera medición.
//
// Dos pruebas, y hacen falta las dos:
//
//   - la de FORMA lee el guion como texto y exige que la sección i sea la que el parser espera en
//     i. No necesita Linux ni /proc, así que corre en cualquier máquina y en cualquier CI.
//   - la REAL corre el guion por una shell de verdad contra ESTE mismo Linux y compara lo parseado
//     con lo que la máquina dice por otra puerta. Es la única que puede cazar un cambio que
//     mantenga el orden del texto y aun así devuelva otra cosa.
// ────────────────────────────────────────────────────────────────────────────────────────────

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

// seccionesDelGuion es EL CONTRATO, escrito una sola vez: qué tiene que leer cada sección y en
// qué campo de LecturasProc la deposita el parser. El índice de la fila es el índice de `tomar()`.
//
// El `marcador` es el pedazo del comando que no puede faltar; si alguien cambia `cat
// /proc/meminfo` por `head -n 40 /proc/meminfo` la prueba sigue pasando, que es lo correcto: lo
// custodiado es el ORDEN y la FUENTE, no la forma exacta de leerla.
var seccionesDelGuion = []struct {
	marcador string // texto que identifica esa lectura dentro del guion
	campo    string // dónde la deposita ParsearLecturaRemota
}{
	{"/proc/stat", "Stat"},
	{"/proc/meminfo", "Meminfo"},
	{"/proc/loadavg", "Loadavg"},
	{"/proc/uptime", "Uptime"},
	{"df -B1 /", "Df"},
	// El marcador es el GLOB, no `thermal_zone0`: desde el 2026-09-05 el guion enumera TODAS
	// las zonas y emite `<type> <miligrados>` por línea, porque leer la 0 fija medía `acpitz`
	// —estático— en vez del paquete de CPU. La posición de la sección NO cambió, que es lo
	// único que esta tabla custodia.
	{"/sys/class/thermal/thermal_zone*/", "TempMil"},
	{"^processor /proc/cpuinfo", "NumCPU"},
	{"ls -1 /proc", "Procs"},
}

// EL ORDEN DE LAS SECCIONES DEL GUION ES EL ORDEN DE LOS ÍNDICES DEL PARSER.
//
// Sabotaje que la hace fallar (VERIFICADO): mover la sección de `ls -1 /proc` al MEDIO del guion
// —por ejemplo a segunda— en vez de dejarla apendeada al final. La prueba se pone roja diciendo
// qué sección esperaba en esa posición y qué encontró. También la ponen roja: intercambiar dos
// secciones cualesquiera, borrar una, o apendear una novena sin darle su índice en el parser.
func TestElOrdenDelGuionEsElOrdenDeLosIndicesDelParser(t *testing.T) {
	// El guion es `cmd0; echo 'SEP'; cmd1; echo 'SEP'; ...; cmdN`. Partirlo por la marca deja
	// exactamente una pieza por sección, en orden: la pieza i lleva el comando de la sección i.
	piezas := strings.Split(guionLecturaProc, separadorProc)
	if len(piezas) != len(seccionesDelGuion) {
		t.Fatalf("el guion tiene %d secciones y el parser lee %d: cada sección nueva se APENDEA al final "+
			"Y se le agrega su `tomar(%d)` en ParsearLecturaRemota, o queda muda",
			len(piezas), len(seccionesDelGuion), len(seccionesDelGuion))
	}
	for i, esperada := range seccionesDelGuion {
		if !strings.Contains(piezas[i], esperada.marcador) {
			t.Errorf("la sección %d del guion no lee %s (el parser la deposita en LecturasProc.%s). "+
				"Lo que hay ahí es: %q. Si agregaste una lectura, APENDEALA al final: insertarla en el medio "+
				"corre todos los índices de tomar() y produce muestras cruzadas sin un solo error",
				i, esperada.marcador, esperada.campo, strings.TrimSpace(piezas[i]))
			continue
		}
		// Y NINGUNA OTRA sección puede estar acá. Sin esto, un guion que lea /proc/stat ocho veces
		// pasaría: lo que se custodia es que la posición i sea la de i y de ninguna otra.
		for j, otra := range seccionesDelGuion {
			if j != i && strings.Contains(piezas[i], otra.marcador) {
				t.Errorf("la sección %d del guion lee también lo de la sección %d (%s): dos lecturas en una "+
					"sección dejan la otra vacía y corren todos los índices que siguen", i, j, otra.marcador)
			}
		}
	}
}

// EL GUION REAL, CORRIDO CONTRA ESTE MISMO LINUX, NO PRODUCE UNA MUESTRA CRUZADA.
//
// La prueba de forma de arriba mira el TEXTO del guion; ésta lo EJECUTA y compara lo parseado con
// lo que esta máquina dice por otra puerta (/proc/cpuinfo leído a mano acá, no por el guion). Es
// la mitad que ningún doble puede simular: la misma lección que TestLoQueLlegaALaShellRemotaEsEjecutable
// aprendió con el `--` de más.
//
// El destino remoto se simula igual que ahí —`sh -c` con el mismo texto que viaja— porque del otro
// lado de ssh y de adb SIEMPRE hay una shell, y el guion es lo único que llega.
//
// Sabotaje que la hace fallar (VERIFICADO): insertar `ls -1 /proc` como SEGUNDA sección del guion
// sin tocar el parser. NumCPU pasa a valer la temperatura en miligrados (27800 en esta máquina),
// MemTotal queda en 0 y Load1 en nil — que es exactamente la muestra basura que se guardaba.
func TestElGuionRealCorridoContraEsteLinuxNoCruzaLasSecciones(t *testing.T) {
	if _, err := os.Stat("/proc/meminfo"); err != nil {
		t.Skip("sin /proc no hay contra qué comparar: esta prueba mide el guion contra la máquina que la corre")
	}
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sin sh no se puede reproducir lo que corre del otro lado")
	}
	salida, err := exec.Command("sh", "-c", guionLecturaProc).Output()
	if err != nil {
		t.Fatalf("el guion no corrió en esta máquina: %v", err)
	}

	l, ok := ParsearLecturaRemota(string(salida))
	if !ok {
		t.Fatal("la salida REAL del guion se rechazó como «no es Linux»: el guion y el parser dejaron de entenderse")
	}

	// (a) EL CONTEO DE CPUs, contra /proc/cpuinfo leído acá y no por el guion. Es el anclaje que
	// caza el corrimiento de índices: con una sección de más al principio, este campo termina
	// leyendo la temperatura en miligrados y da miles.
	if esperado := cpusSegunProcCpuinfo(t); l.NumCPU != esperado {
		t.Errorf("el guion dice %d CPUs y /proc/cpuinfo tiene %d procesadores: los índices de tomar() "+
			"y el orden de las secciones del guion se corrieron", l.NumCPU, esperado)
	}

	// (b) CADA SECCIÓN TRAE LO SUYO. Los marcadores son de CONTENIDO, no del comando: es lo que
	// distingue «la sección está en su lugar» de «el texto del guion está bien escrito».
	for _, c := range []struct{ campo, texto, marcador string }{
		{"Stat", l.Stat, "cpu "},
		{"Meminfo", l.Meminfo, "MemTotal"},
		{"Uptime", l.Uptime, "."}, // dos flotantes; el punto alcanza para no confundirla con otra
	} {
		if !strings.Contains(c.texto, c.marcador) {
			t.Errorf("LecturasProc.%s no contiene %q: la sección que le toca trae otra cosa (%q)",
				c.campo, c.marcador, primerasRunas(c.texto, 80))
		}
	}

	// (c) Y LA MUESTRA ARMADA CON ESO ES CREÍBLE. Acá está el daño de verdad: una muestra cruzada
	// pasa `Valida()` sin quejarse —son todos ceros plausibles— y se guarda y se dibuja.
	m := MuestraDesde(l, nil)
	if m.MemTotal == 0 {
		t.Error("MemTotal = 0 con el guion real: la sección de meminfo no llegó a su campo")
	}
	if m.DiscoTotal == 0 {
		t.Error("DiscoTotal = 0 con el guion real: la sección de df no llegó a su campo")
	}
	if m.Load1 == nil {
		t.Error("Load1 = nil con el guion real: la sección de loadavg no llegó a su campo")
	}
	if m.UptimeSeg <= 0 {
		t.Errorf("UptimeSeg = %d con el guion real: la sección de uptime no llegó a su campo", m.UptimeSeg)
	}
	if m.NumProcesos <= 0 {
		t.Error("NumProcesos = 0 con el guion real: la octava sección (`ls -1 /proc`) no llegó a su campo")
	}
	// Ninguna máquina viva tiene un proceso solo: un 1 es el síntoma de que este campo se está
	// llenando con otra sección (el conteo de CPUs, el uptime), que es el modo de falla real.
	if m.NumProcesos == 1 {
		t.Error("NumProcesos = 1: eso no es un listado de /proc, es otra sección contada como si lo fuera")
	}
	if err := m.Valida(); err != nil {
		t.Errorf("la muestra del guion real no valida: %v", err)
	}
}

// cpusSegunProcCpuinfo cuenta los procesadores SIN pasar por el guion, que es lo que la convierte
// en un anclaje. Contra runtime.NumCPU() no serviría: ése respeta la afinidad del proceso y en un
// contenedor con CPUs acotadas da menos que /proc/cpuinfo, y la prueba fallaría por algo que no es
// el bug.
func cpusSegunProcCpuinfo(t *testing.T) int {
	t.Helper()
	b, err := os.ReadFile("/proc/cpuinfo")
	if err != nil {
		t.Skipf("no se pudo leer /proc/cpuinfo para comparar: %v", err)
	}
	n := 0
	for _, linea := range strings.Split(string(b), "\n") {
		if strings.HasPrefix(linea, "processor") {
			n++
		}
	}
	if n == 0 {
		t.Skip("/proc/cpuinfo sin líneas `processor` (arquitectura no x86): no hay contra qué comparar")
	}
	return n
}

func primerasRunas(s string, n int) string {
	rs := []rune(strings.TrimSpace(s))
	if len(rs) <= n {
		return string(rs)
	}
	return string(rs[:n]) + "…"
}
