package fleet

import (
	"strings"
	"testing"
)

// Fixtures reales, capturados de esta máquina el 2026-08-26. Que sean REALES importa: un
// /proc/meminfo inventado no tiene las 50 líneas que hay que saltear, ni el formato exacto.
const (
	statReal = `cpu  931771 2364 439752 8613590 173159 0 7588 0 0 0
cpu0 77647 197 36646 717799 14430 0 632 0 0 0
intr 12345678 0 0
ctxt 987654321`

	meminfoReal = `MemTotal:        7843996 kB
MemFree:         1204168 kB
MemAvailable:    4745104 kB
Buffers:          123456 kB
Cached:          2345678 kB
SwapCached:            0 kB
SwapTotal:       9941140 kB
SwapFree:        8404964 kB`

	dfReal = `Filesystem      1B-blocks        Used   Available Use% Mounted on
/dev/nvme0n1p3 502392610816 85868347392 391000000000  18% /`
)

// EL MISMO PARSEO SIRVE PARA LAS TRES FUENTES: el /proc local, el de un Tier B por SSH y el de un
// Android por ADB. Ésta es la prueba que lo cubre para las dos que no se pueden correr desde acá.
func TestElParseoDeProcProduceLosMismosNumerosVengaDeDondeVenga(t *testing.T) {
	l := LecturasProc{
		Stat: statReal, Meminfo: meminfoReal, Loadavg: "1.64 2.33 2.31 5/1181 94909",
		Uptime: "8623.50 86136.11", Df: dfReal, TempMil: "27800", NumCPU: 12,
	}
	var cpu contadorCPU
	cpu.delta(1000, 2000) // una lectura previa, para que haya derivada
	m := MuestraDesde(l, &cpu)

	if m.NumCPU != 12 {
		t.Errorf("NumCPU = %d", m.NumCPU)
	}
	// Memoria: usada = total - MemAvailable, en bytes.
	if quiero := uint64(7843996-4745104) * 1024; m.MemUsada != quiero {
		t.Errorf("MemUsada = %d, esperaba %d (total menos MemAvailable)", m.MemUsada, quiero)
	}
	if quiero := uint64(7843996) * 1024; m.MemTotal != quiero {
		t.Errorf("MemTotal = %d, esperaba %d", m.MemTotal, quiero)
	}
	if quiero := uint64(9941140-8404964) * 1024; m.SwapUsada != quiero {
		t.Errorf("SwapUsada = %d, esperaba %d", m.SwapUsada, quiero)
	}
	// Carga.
	if m.Load1 == nil || *m.Load1 != 1.64 {
		t.Errorf("Load1 = %v, esperaba 1.64", m.Load1)
	}
	if m.UptimeSeg != 8623 {
		t.Errorf("UptimeSeg = %d, esperaba 8623", m.UptimeSeg)
	}
	// Disco: las TRES columnas de df, y NO suman el total (la reserva de root).
	if m.DiscoTotal != 502392610816 || m.DiscoUsado != 85868347392 || m.DiscoDisponible != 391000000000 {
		t.Errorf("disco mal parseado: total=%d usado=%d disp=%d", m.DiscoTotal, m.DiscoUsado, m.DiscoDisponible)
	}
	if m.DiscoUsado+m.DiscoDisponible >= m.DiscoTotal {
		t.Error("usado + disponible >= total: se perdió la reserva de root, así que uno se derivó del otro")
	}
	if m.TempC == nil || *m.TempC != 27.8 {
		t.Errorf("TempC = %v, esperaba 27.8", m.TempC)
	}
	if err := m.Valida(); err != nil {
		t.Errorf("la muestra parseada no pasa Valida(): %v", err)
	}
}

// La memoria usada sale de MemAvailable y NO de MemFree, también en el camino remoto.
//
// Sabotaje que la hace fallar: usar MemFree en ParsearMeminfo. Con estos números la diferencia es
// de 3,5 GB — un Linux sano aparecería al 85 % en vez del 40 %.
func TestElParseoRemotoTampocoUsaMemFree(t *testing.T) {
	var m Muestra
	ParsearMeminfo(meminfoReal, &m)
	conAvailable := uint64(7843996-4745104) * 1024
	conFree := uint64(7843996-1204168) * 1024
	if m.MemUsada == conFree {
		t.Fatalf("se usó MemFree (%d): un Linux sano usa toda la RAM libre como page cache y aparecería al 85%% para siempre", conFree)
	}
	if m.MemUsada != conAvailable {
		t.Errorf("MemUsada = %d, esperaba %d", m.MemUsada, conAvailable)
	}
}

// Lo que no vino queda en nil o en cero-por-par, nunca en un número inventado.
func TestUnaLecturaIncompletaNoInventaNumeros(t *testing.T) {
	// Un router que responde al ssh y sólo tiene /proc/uptime.
	m := MuestraDesde(LecturasProc{Uptime: "500.0 900.0"}, nil)
	if m.UptimeSeg != 500 {
		t.Errorf("UptimeSeg = %d", m.UptimeSeg)
	}
	if m.CPUPct != nil || m.Load1 != nil || m.TempC != nil {
		t.Errorf("se inventaron valores que no vinieron: cpu=%v load=%v temp=%v", m.CPUPct, m.Load1, m.TempC)
	}
	// REGLA DE LOS PARES: sin meminfo no se fija ni total ni usado.
	if m.MemTotal != 0 || m.MemUsada != 0 || m.DiscoTotal != 0 {
		t.Errorf("se fijaron totales sin haberlos medido: mem=%d disco=%d", m.MemTotal, m.DiscoTotal)
	}
	if err := m.Valida(); err != nil {
		t.Errorf("una muestra incompleta debería ser válida (todo nil/cero): %v", err)
	}

	// EL CASO QUE DE VERDAD EJERCITA LA REGLA DE LOS PARES, y que la primera versión de esta
	// prueba NO cubría: un meminfo que trae el TOTAL y no trae con qué calcular el usado.
	//
	// Pasa de verdad: un Android viejo, un busybox recortado, un /proc/meminfo truncado. Sin la
	// regla se fijaría MemTotal con MemUsada en 0, y el panel diría «esta máquina no usa memoria»
	// — el cero mentiroso contra el que existe todo el diseño.
	//
	// Sabotaje que la hace fallar: fijar m.MemTotal fuera del `if` que exige el disponible.
	var parcial Muestra
	ParsearMeminfo("MemTotal:        7843996 kB\nBuffers:          123456 kB", &parcial)
	if parcial.MemTotal != 0 || parcial.MemUsada != 0 {
		t.Errorf("con MemTotal pero SIN MemAvailable ni MemFree se fijó total=%d usado=%d: "+
			"un total sin su usado se lee como 0%% de memoria en uso", parcial.MemTotal, parcial.MemUsada)
	}
	// Y con MemFree (kernels viejos) sí se puede: ahí el par está completo.
	var viejo Muestra
	ParsearMeminfo("MemTotal:        1000 kB\nMemFree:          400 kB", &viejo)
	if viejo.MemTotal == 0 || viejo.MemUsada != 600*1024 {
		t.Errorf("un kernel viejo con MemFree debería medirse: total=%d usado=%d", viejo.MemTotal, viejo.MemUsada)
	}
}

// La lectura remota se parte en secciones, y una salida que NO es de un Linux se rechaza en vez
// de producir una muestra de ceros.
//
// Sabotaje: devolver ok=true siempre → un router de firmware propietario que responde al ssh
// aparecería con 0 % de todo, y ese cero se cree.
func TestUnaSalidaQueNoEsLinuxSeRechaza(t *testing.T) {
	sep := separadorProc
	completa := statReal + "\n" + sep + "\n" + meminfoReal + "\n" + sep + "\n1.0 2.0 3.0\n" + sep +
		"\n100.0 200.0\n" + sep + "\n" + dfReal + "\n" + sep + "\n27800\n" + sep + "\n12"
	l, ok := ParsearLecturaRemota(completa)
	if !ok {
		t.Fatal("una lectura completa se rechazó")
	}
	if l.NumCPU != 12 || !strings.Contains(l.Meminfo, "MemAvailable") {
		t.Errorf("secciones mal partidas: %+v", l)
	}

	for _, basura := range []string{
		"",
		"-ash: cat: not found",
		"Welcome to the router CLI\n>",
	} {
		if _, ok := ParsearLecturaRemota(basura); ok {
			t.Errorf("se aceptó como Linux una salida que no lo es: %q", basura)
		}
	}
}

// `df` puede partir el nombre del dispositivo en dos líneas cuando es largo. El parseo busca
// tres números seguidos en vez de posiciones fijas.
func TestElParseoDeDfAguantaElNombreLargo(t *testing.T) {
	partido := `Filesystem     1B-blocks  Used Available Use% Mounted on
/dev/mapper/un-nombre-de-volumen-logico-larguisimo
              502392610816 85868347392 391000000000  18% /`
	var m Muestra
	ParsearDf(partido, &m)
	if m.DiscoTotal != 502392610816 || m.DiscoUsado != 85868347392 {
		t.Errorf("df partido en dos líneas mal parseado: total=%d usado=%d", m.DiscoTotal, m.DiscoUsado)
	}
}

// El techo de iOS se reconoce por el `os` declarado, en cualquiera de sus formas.
func TestSeReconoceIOSParaDecirLaVerdadTemprano(t *testing.T) {
	for _, si := range []string{"ios", "iOS", " iPadOS ", "iphone 15", "ipad-pro"} {
		if !EsIOS(si) {
			t.Errorf("EsIOS(%q) = false", si)
		}
	}
	for _, no := range []string{"android", "linux", "windows", "darwin", ""} {
		if EsIOS(no) {
			t.Errorf("EsIOS(%q) = true", no)
		}
	}
	// Y el mensaje explica que el techo es de la PLATAFORMA, no de Musubi.
	if !strings.Contains(ErrIOSNoSeMide.Error(), "MDM") || !strings.Contains(strings.ToLower(ErrIOSNoSeMide.Error()), "no es una limitación de musubi") {
		t.Errorf("el mensaje no explica de quién es el techo: %v", ErrIOSNoSeMide)
	}
}

// Los fallos de adb que vienen como TEXTO (no como exit code) se traducen a algo accionable.
// El de `unauthorized` es EL importante: el problema está en la mano de alguien, no en la red.
func TestLosFallosDeADBSeTraducen(t *testing.T) {
	casos := []struct{ salida, espera string }{
		{"error: device unauthorized.", "EN LA PANTALLA"},
		{"error: device 'X' not found", "adb connect"},
		{"error: more than one device/emulator", "serial exacto"},
	}
	for _, c := range casos {
		got := detectarFalloADB("mi-tele", c.salida)
		if got == "" {
			t.Errorf("no se tradujo %q", c.salida)
			continue
		}
		if !strings.Contains(got, c.espera) {
			t.Errorf("la traducción de %q no dice qué hacer (%q): %s", c.salida, c.espera, got)
		}
	}
	// Una salida normal no se confunde con un fallo.
	if got := detectarFalloADB("x", "MemTotal: 123 kB"); got != "" {
		t.Errorf("una salida normal se leyó como fallo: %s", got)
	}
}
