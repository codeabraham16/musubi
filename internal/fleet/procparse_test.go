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
	// I3 — MemLibre ES MemFree, no otra cosa: el número crudo del fixture, en bytes.
	// Sabotaje: no asignar MemFree en ParsearMeminfo (queda nil y esta rama lo dice).
	if m.MemLibre == nil {
		t.Error("MemLibre = nil con un meminfo que trae MemFree: no se está absorbiendo")
	} else if quiero := uint64(1204168) * 1024; *m.MemLibre != quiero {
		t.Errorf("MemLibre = %d, esperaba %d (el MemFree del fixture, en bytes)", *m.MemLibre, quiero)
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

	// AHORA QUE MemFree VIVE EN LA STRUCT, el atajo está a un campo de distancia: MemUsada
	// «tiene» que ser MemTotal - MemLibre, dice la intuición. No lo es, y la distancia entre las
	// dos cuentas se mide en GIGABYTES, no en ruido.
	//
	// Sabotaje: en ParsearMeminfo, calcular MemUsada con vals["MemFree"] en vez de con
	// `disponible`.
	if m.MemLibre == nil {
		t.Fatal("MemLibre = nil: sin el campo, esta guarda no custodia nada")
	}
	if m.MemUsada == m.MemTotal-*m.MemLibre {
		t.Fatalf("MemUsada (%d) es exactamente total menos MemLibre: se derivó del campo equivocado", m.MemUsada)
	}
	const gigabyte = 1 << 30
	if d := (m.MemTotal - *m.MemLibre) - m.MemUsada; d < 3*gigabyte {
		t.Errorf("entre la cuenta con MemAvailable (%d) y la cuenta con MemFree (%d) hay %d bytes: "+
			"con este fixture tienen que ser ~3,5 GB (85%% contra 40%% de RAM usada). Si la distancia se achicó, "+
			"una de las dos cuentas cambió", m.MemUsada, m.MemTotal-*m.MemLibre, d)
	}
}

// I4 — EL CONTEO DE PROCESOS NO SALE DEL 4º CAMPO DE /proc/loadavg.
//
// Es la única prueba que caza esta trampa, y hace falta porque la trampa NO produce un error:
// produce un número creíble. En «1.64 2.33 2.31 5/1181 94909» el 1181 son los HILOS del sistema
// y el 5 los ejecutables en ese instante; ninguno de los dos es la cantidad de procesos, y el
// dato ya está leído y a mano en LecturasProc.Loadavg.
//
// Sabotaje que la hace fallar: derivar NumProcesos del 4º campo de Loadavg (el denominador, 1181,
// o el numerador, 5) en vez de contar el listado de /proc.
func TestElConteoDeProcesosNoSaleDelCuartoCampoDeLoadavg(t *testing.T) {
	// Siete pids de mentira, y de paso la basura que /proc tiene al lado.
	listado := "1\n2\n847\n1024\n1025\n9931\n94909\nself\nthread-self\ncpuinfo\nmeminfo\nnet\n"
	l := LecturasProc{
		Meminfo: meminfoReal,
		Loadavg: "1.64 2.33 2.31 5/1181 94909",
		Procs:   listado,
	}
	m := MuestraDesde(l, nil)

	if m.NumProcesos != 7 {
		t.Errorf("NumProcesos = %d, esperaba 7 (los pids del listado)", m.NumProcesos)
	}
	if m.NumProcesos == 1181 {
		t.Error("NumProcesos = 1181: ése es el denominador del 4º campo de loadavg y cuenta HILOS, " +
			"no procesos — da entre 3 y 5 veces más y no falla nunca, sólo miente")
	}
	if m.NumProcesos == 5 {
		t.Error("NumProcesos = 5: ése es el numerador del 4º campo de loadavg (los ejecutables en este " +
			"instante), no la cantidad de procesos del sistema")
	}
}

// I5 — ContarPids cuenta PROCESOS, no cualquier entrada de /proc.
//
// El filtro tiene que ser «el nombre es TODO dígitos». Los parecidos fallan de maneras distintas
// y todas silenciosas: un HasPrefix numérico deja pasar "12ab", y filtrar por ausencia de "/" no
// filtra nada.
//
// Sabotaje: cambiar el filtro por strings.HasPrefix con un dígito, o por «no contiene /».
func TestContarPidsIgnoraLoQueNoEsUnPid(t *testing.T) {
	if n := ContarPids("1\n2\n1234\nself\nthread-self\ncpuinfo\nnet\n12ab\n"); n != 3 {
		t.Errorf("ContarPids = %d, esperaba 3: sólo 1, 2 y 1234 son pids ("+
			"«12ab» empieza con dígitos y NO lo es)", n)
	}
	// Un listado vacío es «no medido», no «cero procesos»: devuelve 0 y quien lo consume lo
	// traduce a null.
	if n := ContarPids(""); n != 0 {
		t.Errorf("ContarPids(\"\") = %d, esperaba 0", n)
	}
	// Espacios sueltos y líneas en blanco no cuentan.
	if n := ContarPids("\n  \n 42 \n"); n != 1 {
		t.Errorf("ContarPids con espacios = %d, esperaba 1", n)
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
	// I9 — los dos campos nuevos hablan el mismo idioma del «no sé»: nil el puntero, 0 el entero.
	// Sabotaje: inicializar MemLibre con u64(0) cuando no hay MemFree — un 0 se lee «no le queda
	// nada de RAM libre», que es lo contrario de «no lo sé».
	if m.MemLibre != nil {
		t.Errorf("MemLibre = %d sin haber leído meminfo: tiene que ser nil", *m.MemLibre)
	}
	if m.NumProcesos != 0 {
		t.Errorf("NumProcesos = %d sin listado de /proc: tiene que ser 0 (= no medido)", m.NumProcesos)
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
	// Y ACÁ, SÓLO ACÁ, `MemUsada == MemTotal - MemLibre` ES LEGÍTIMO, y queda escrito para que
	// nadie lo «arregle»: en el camino del kernel viejo no hay MemAvailable contra qué restar, así
	// que las dos cuentas coinciden POR CONSTRUCCIÓN. En el camino normal esa igualdad es el bug.
	if viejo.MemLibre == nil || viejo.MemUsada != viejo.MemTotal-*viejo.MemLibre {
		t.Errorf("en el camino de kernel viejo las dos cuentas tienen que coincidir: usada=%d total=%d libre=%v",
			viejo.MemUsada, viejo.MemTotal, viejo.MemLibre)
	}
}

// I12 — UNA MUESTRA GUARDADA VIEJA SIGUE LEYÉNDOSE, y lo que le falta se lee como «no medido».
//
// Es el despliegue escalonado: el cerebro se actualiza primero y durante días recibe latidos de
// agentes viejos, además de tener en la columna `last_sample` muestras guardadas antes de este
// slice. Ninguna de las dos trae mem_libre ni num_procesos, y eso NO es un error.
//
// Sabotaje: hacer que MuestraDesdeTexto exija los campos (o que el JSON los rellene con ceros al
// deserializar, que es el mismo bug con otra cara).
func TestUnaMuestraSinLosCamposNuevosSeLeeComoNoMedida(t *testing.T) {
	vieja := `{"tomada":"2026-08-01T10:00:00Z","cpu_pct":12.5,"num_cpu":8,` +
		`"mem_total":8589934592,"mem_usada":4294967296,"swap_total":0,"swap_usada":0,` +
		`"disco_total":1000,"disco_usado":100,"disco_disponible":850,` +
		`"load1":1.5,"load5":1.2,"load15":1.1,"uptime_seg":3600,"temp_c":null}`
	m, err := MuestraDesdeTexto(vieja)
	if err != nil {
		t.Fatalf("una muestra guardada antes de este slice dejó de leerse: %v", err)
	}
	if m == nil {
		t.Fatal("MuestraDesdeTexto devolvió nil sin error")
	}
	if m.MemLibre != nil {
		t.Errorf("MemLibre = %d en una muestra que no lo traía: tiene que ser nil", *m.MemLibre)
	}
	if m.NumProcesos != 0 {
		t.Errorf("NumProcesos = %d en una muestra que no lo traía: tiene que ser 0", m.NumProcesos)
	}
	// Y lo que SÍ traía sigue intacto: no se perdió nada en el camino.
	if m.NumCPU != 8 || m.MemUsada != 4294967296 {
		t.Errorf("la muestra vieja se leyó mal: %+v", m)
	}
	if err := m.Valida(); err != nil {
		t.Errorf("una muestra vieja dejó de ser válida: %v", err)
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

	// I6 — LA OCTAVA SECCIÓN SE APENDEÓ, y las dos formas tienen que parsear.
	//
	// La de arriba es la salida VIEJA, de siete secciones: un guion anterior a este slice, o un
	// Tier B al que todavía no le llegó. Tiene que seguir dando ok=true, con NumProcesos en 0.
	if n := ContarPids(l.Procs); n != 0 {
		t.Errorf("una salida vieja de 7 secciones reportó %d procesos: tomar(7) tiene que dar \"\"", n)
	}

	// Y la NUEVA, de ocho.
	//
	// EL TECHO DE ESTA PRUEBA, ESCRITO PARA QUE NADIE SE CONFÍE: acá el texto de entrada lo arma
	// ELLA MISMA, así que verifica el PARSER y no el guion. El sabotaje que declaraba antes
	// —«insertar `ls -1 /proc` en el medio del guion»— NO la ponía roja: el guion no se toca en
	// este archivo. Ese contrato lo custodia guion_remoto_test.go, y ahí sí se pone rojo.
	//
	// Sabotaje que la hace fallar (VERIFICADO): correr los índices de `tomar()` en
	// ParsearLecturaRemota —leer NumCPU de tomar(5) y Procs de tomar(6), por ejemplo—. Ahí la
	// memoria se lee como carga y la temperatura como conteo de procesadores.
	conProcs := completa + "\n" + sep + "\n1\n2\n1234\nself\ncpuinfo\n"
	l8, ok := ParsearLecturaRemota(conProcs)
	if !ok {
		t.Fatal("la salida NUEVA de 8 secciones se rechazó")
	}
	if l8.NumCPU != 12 {
		t.Errorf("NumCPU = %d con 8 secciones: los índices se corrieron", l8.NumCPU)
	}
	if !strings.Contains(l8.Meminfo, "MemAvailable") || !strings.HasPrefix(strings.TrimSpace(l8.Loadavg), "1.0") {
		t.Errorf("con 8 secciones se cruzaron los índices: meminfo=%q loadavg=%q", l8.Meminfo, l8.Loadavg)
	}
	if n := ContarPids(l8.Procs); n != 3 {
		t.Errorf("la octava sección dio %d procesos, esperaba 3: %q", n, l8.Procs)
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
