package fleet

// Pruebas del SEAM de colectores, independientes de la plataforma.

import (
	"runtime"
	"testing"
	"time"
)

// TODA plataforma tiene un colector, y ninguna devuelve una Muestra de ceros haciéndose pasar por
// una medición. Ésta es la prueba que corre en las tres y las obliga a ser honestas.
//
// Sabotaje que la hace fallar: que un colector devuelva `Muestra{}, nil` en vez de un error, que
// es exactamente el atajo que pintaría toda una flota al 0 % de CPU.
func TestTodaPlataformaTieneColectorYNingunoMienteConCeros(t *testing.T) {
	c := NuevoColector()
	if c == nil {
		t.Fatal("NuevoColector devolvió nil: alguna plataforma se quedó sin seam")
	}
	m, err := c.Tomar()

	if err != nil {
		// Plataforma sin colector: el error tiene que ser el declarado, y la muestra tiene que
		// venir VACÍA (nadie debe confundirla con una medición parcial).
		if err != ErrSinColector {
			t.Errorf("error inesperado: %v", err)
		}
		if m.NumCPU != 0 || m.MemTotal != 0 || m.DiscoTotal != 0 {
			t.Errorf("un colector que falla devolvió datos: %+v", m)
		}
		return
	}

	// Plataforma CON colector: lo mínimo que cualquiera puede saber.
	if m.Tomada.IsZero() {
		t.Error("la muestra no lleva la hora en que se tomó")
	}
	if m.NumCPU < 1 {
		t.Errorf("NumCPU = %d en %s", m.NumCPU, runtime.GOOS)
	}
	// Y lo medido tiene que pasar su propia validación, incluida la regla de los pares.
	if err := m.Valida(); err != nil {
		t.Errorf("la muestra de %s no pasa Valida(): %v", runtime.GOOS, err)
	}
}

// LA REGLA DE LOS PARES, verificada sobre lo que REALMENTE mide esta plataforma.
//
// Existe porque la tentación es concreta: macOS da hw.memsize por sysctl y NO da la memoria
// usada sin mach, así que fijar sólo el total es un paso natural — y produce un 0 % que se lee
// como «esta máquina no usa memoria».
//
// Sabotaje: fijar MemTotal sin MemUsada en cualquier colector.
func TestLaReglaDeLosParesSeRespetaEnEstaPlataforma(t *testing.T) {
	m, err := NuevoColector().Tomar()
	if err != nil {
		t.Skipf("sin colector en %s", runtime.GOOS)
	}
	// EL SWAP QUEDA FUERA DE LA MITAD «total sin usado», y la razón es que esta prueba SE
	// EQUIVOCÓ primero y vale dejarlo escrito.
	//
	// La regla busca cazar «fijé el total y no medí el usado», que produce un 0 % engañoso. Pero
	// al nivel del VALOR, «no lo medí» y «medí y da cero» son indistinguibles — que es exactamente
	// la confusión contra la que existe todo este diseño, y la guarda cayó en ella. En memoria y
	// disco no importa: en una máquina encendida, `usado == 0` es imposible, así que un cero ahí
	// sólo puede ser un no-medido. En SWAP es COMÚN Y LEGÍTIMO: un swap vacío es un swap sano.
	//
	// Lo cazó la máquina misma: la prueba pasó durante horas y empezó a fallar cuando el swap se
	// liberó. Una prueba que depende del estado del entorno para tener razón no la tenía.
	pares := []struct {
		nombre       string
		total, usado uint64
		ceroPosible  bool
	}{
		{"memoria", m.MemTotal, m.MemUsada, false},
		{"disco", m.DiscoTotal, m.DiscoUsado, false},
		{"swap", m.SwapTotal, m.SwapUsada, true}, // un swap vacío es un swap sano
	}
	for _, p := range pares {
		if !p.ceroPosible && p.total > 0 && p.usado == 0 {
			t.Errorf("%s: total=%d con usado=0 — en una máquina encendida eso no se mide, se olvida, "+
				"y produce un 0%% engañoso", p.nombre, p.total)
		}
		if p.usado > p.total {
			t.Errorf("%s: usado=%d supera total=%d", p.nombre, p.usado, p.total)
		}
	}
}

// LO QUE CADA PLATAFORMA PUEDE Y NO PUEDE MEDIR, ESCRITO.
//
// No es documentación: es una prueba. Si algún día llega el colector de mach para macOS y empieza
// a reportar CPU, esta tabla FALLA y obliga a actualizarla — que es cómo la promesa del track
// («qué mide cada sistema») deja de vivir sólo en un README que nadie relee.
func TestLoQueCadaPlataformaMideEstaDeclarado(t *testing.T) {
	m, err := NuevoColector().Tomar()
	if err != nil {
		t.Skipf("sin colector en %s", runtime.GOOS)
	}
	// Segunda lectura: el porcentaje de CPU es una derivada y la primera nunca lo trae.
	//
	// Hay que QUEMAR CPU en el medio, y la primera versión de esta prueba no lo hacía: dos
	// lecturas dentro del mismo tick del reloj del kernel devuelven los MISMOS contadores, así
	// que el delta es cero y `delta` responde nil (correctamente — dividir daría NaN). El test
	// culpaba al colector de no medir cuando el problema era que no había pasado nada que medir.
	c := NuevoColector()
	_, _ = c.Tomar()
	quemarCPU(60 * time.Millisecond)
	m2, _ := c.Tomar()

	type capacidad struct{ cpu, carga, memoria, disco, tempPosible, procesos, memLibre bool }
	esperado := map[string]capacidad{
		// tempPosible: si este SO tiene un CAMINO para leer la temperatura, no si ESTA máquina la
		// expone — eso depende del hardware. El campo se llamaba `temp`, decía `false` en las tres
		// filas «porque depende del hardware» y NO SE COMPROBABA EN NINGUNA PARTE: una columna de
		// una tabla que el propio slice declara como «una PRUEBA, no un README», y que era un
		// README. Se notó al agregar el colector de Windows (A2): la fila quedó vieja y la suite
		// siguió verde. Lo que SÍ se puede afirmar es la mitad negativa — ver abajo.
		// memLibre: sólo Linux la tiene sin mentir. En Windows el candidato es
		// MEMORYSTATUSEX.ullAvailPhys, que es el análogo de MemAvailable y NO de MemFree; en
		// macOS haría falta host_statistics64 (mach). En los dos, medir de más sería mentir.
		// procesos: Linux cuenta /proc, Windows usa K32GetPerformanceInfo, y macOS necesitaría un
		// fork+exec por latido en el proceso que corre en todas las máquinas.
		"linux":   {cpu: true, carga: true, memoria: true, disco: true, tempPosible: true, procesos: true, memLibre: true},
		"windows": {cpu: true, carga: false, memoria: true, disco: true, tempPosible: true, procesos: true, memLibre: false},
		"darwin":  {cpu: false, carga: true, memoria: false, disco: true, tempPosible: false, procesos: false, memLibre: false},
	}
	quiero, hay := esperado[runtime.GOOS]
	if !hay {
		t.Skipf("%s no está en la tabla de capacidades declaradas", runtime.GOOS)
	}

	if got := m2.CPUPct != nil; got != quiero.cpu {
		t.Errorf("%s: mide CPU = %v, la tabla dice %v — actualizá la tabla o el colector", runtime.GOOS, got, quiero.cpu)
	}
	if got := m.Load1 != nil; got != quiero.carga {
		t.Errorf("%s: mide carga = %v, la tabla dice %v", runtime.GOOS, got, quiero.carga)
	}
	if got := m.MemTotal > 0; got != quiero.memoria {
		t.Errorf("%s: mide memoria = %v, la tabla dice %v", runtime.GOOS, got, quiero.memoria)
	}
	if got := m.DiscoTotal > 0; got != quiero.disco {
		t.Errorf("%s: mide disco = %v, la tabla dice %v", runtime.GOOS, got, quiero.disco)
	}
	if got := m.NumProcesos > 0; got != quiero.procesos {
		t.Errorf("%s: cuenta procesos = %v, la tabla dice %v — actualizá la tabla o el colector",
			runtime.GOOS, got, quiero.procesos)
	}
	// EL QUE MÁS IMPORTA DE LOS DOS NUEVOS, porque el atajo está a un campo de distancia.
	//
	// Sabotaje que la hace fallar, corriendo esta prueba en Linux: borrar la asignación de
	// MemFree en ParsearMeminfo (linux pasa a medir mem_libre = false y la tabla dice true).
	//
	// Y el sabotaje que esta fila existe para cazar, que SÓLO se ve corriendo en Windows: emitir
	// MEMORYSTATUSEX.ullAvailPhys como mem_libre. Es MemAvailable disfrazado —incluye la standby
	// list, que es cache reutilizable—, así que sería cometer en Windows la misma confusión que el
	// repo peleó en Linux, y en un archivo que casi nadie puede correr localmente. La tabla es lo
	// único que lo caza, y por eso la fila de windows dice false y no «no sé».
	if got := m.MemLibre != nil; got != quiero.memLibre {
		t.Errorf("%s: mide mem_libre = %v, la tabla dice %v — si es Windows o macOS, lo que se está "+
			"reportando NO es MemFree sino su primo el «disponible», y son cosas distintas",
			runtime.GOOS, got, quiero.memLibre)
	}

	// LA TEMPERATURA SE AFIRMA POR LA MITAD QUE SÍ DEPENDE DEL SISTEMA OPERATIVO.
	//
	// «Esta máquina reporta temperatura» no se puede exigir: depende de si el firmware expone el
	// sensor, y un runner de CI casi nunca lo hace. Por eso la fila anterior decía `false` en las
	// tres y no comprobaba nada — con lo cual la columna no significaba nada.
	//
	// Lo que SÍ es un hecho del SO es si existe un CAMINO para leerla, y de ahí salen las dos
	// mitades que se pueden afirmar:
	//
	//	tempPosible=false  ->  el campo tiene que ser nil SIEMPRE. Es la mitad que caza un colector
	//	                       nuevo que empieza a emitir algo sin que nadie actualice la tabla.
	//	tempPosible=true   ->  puede ser nil (sin sensor, el caso común) pero si trae valor tiene
	//	                       que ser PLAUSIBLE. Es la mitad que caza una unidad mal leída — los
	//	                       −243 °C de restar Kelvin sin dividir por 10 pasarían por acá.
	//
	// Sabotaje que la hace fallar: poner tempPosible:true en darwin, o devolver un valor fuera de
	// rango desde cualquier colector.
	if !quiero.tempPosible {
		if m.TempC != nil {
			t.Errorf("%s: reportó temperatura (%.1f °C) y la tabla dice que este SO no tiene cómo leerla — "+
				"o se agregó un colector y nadie actualizó la tabla, o se está emitiendo otra cosa",
				runtime.GOOS, *m.TempC)
		}
	} else if m.TempC != nil {
		if c := *m.TempC; c <= 0 || c > TempMaxPlausibleC {
			t.Errorf("%s: reportó %.1f °C, que está fuera de rango. Un negativo grande es la firma de "+
				"una unidad mal leída (Kelvin sin dividir), no de una máquina fría", runtime.GOOS, c)
		}
	}
}

// quemarCPU gasta ciclos durante d. La usan las pruebas que necesitan que los contadores de CPU
// del kernel AVANCEN entre dos lecturas: sin trabajo real de por medio no hay delta que medir.
func quemarCPU(d time.Duration) {
	fin := time.Now().Add(d)
	x := 0
	for time.Now().Before(fin) {
		x++
	}
	_ = x
}
