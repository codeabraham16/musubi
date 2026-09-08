package mcp

import (
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"musubi/internal/config"
	"musubi/internal/embedding"
	"musubi/internal/memory/memtest"
)

// EL CEREBRO CENTRAL MANTIENE SU PROPIA MEMORIA.
//
// Medido en el central el 2026-09-07: el ciclo SÍ corría, pero lo corría OTRO proceso. La cadena
// completa es `musubi-gateway.service` → `main.py` → `/usr/local/bin/musubi daemon`, y ese daemon
// abre la misma base y sí arranca los cuatro schedulers. O sea que la memoria del cerebro se
// mantenía como efecto secundario de que un bot de chat estuviera vivo; pararlo —una operación
// perfectamente razonable— dejaba la memoria sin consolidar, sin olvidar y sin purgar.
//
// `runServe` ya recibía `WithMaintenance(cfg.Maintenance)`: la config estaba cableada desde
// siempre y el consumidor no existía. No hay ningún comentario que lo excluya a propósito.

// valorDeGauge saca el valor de una métrica sin etiquetas del texto de /metrics.
func valorDeGauge(t *testing.T, salida, metrica string) (int64, bool) {
	t.Helper()
	re := regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(metrica) + ` (-?\d+)$`)
	m := re.FindStringSubmatch(salida)
	if m == nil {
		return 0, false
	}
	v, err := strconv.ParseInt(m[1], 10, 64)
	if err != nil {
		t.Fatalf("el valor de %s no es un entero: %q", metrica, m[1])
	}
	return v, true
}

// M1 — «NUNCA CORRIÓ» NO ES «CORRIÓ RECIÉN».
//
// Es el invariante que hace útil a la serie. Con un entero pelado los dos casos serían 0, y son
// las dos respuestas más distintas posibles: una dice que el ciclo funciona y la otra que nunca
// arrancó. Es la misma convención de `musubi_backup_local_age_seconds`, y la misma clase de
// defecto que este repo persigue —el valor de fallo que se lee como tranquilizador—.
func TestM1UnCicloQueNuncaCorrioNoSeVeIgualQueUnoRecien(t *testing.T) {
	eng := memtest.NuevoEngine(t, t.TempDir())

	// DOS SERVIDORES SOBRE EL MISMO MOTOR, y no uno renderizando dos veces: los gauges de dominio
	// van por un cache TTL (renderDomainGauges) para no repetir los COUNT O(n) en cada scrape, así
	// que un segundo render del MISMO servidor devolvería lo que se cacheó antes de marcar. El
	// cache es correcto en producción; lo que hay que evitar es escribir una prueba que lo ignore.
	antes := NewMcpServer(eng, t.TempDir(), embedding.NoopProvider{})
	nunca, hay := valorDeGauge(t, antes.metrics.render(antes.engine), "musubi_maintenance_age_seconds")
	if !hay {
		t.Fatal("la serie no se exporta: un cerebro que dejó de mantenerse se vería igual que uno sano")
	}
	if nunca != -1 {
		t.Errorf("sin ninguna corrida la serie vale %d, esperaba -1: un 0 diría «corrió recién»", nunca)
	}

	if err := eng.MarkMaintenanceNow(); err != nil {
		t.Fatalf("marcar el mantenimiento: %v", err)
	}

	despues := NewMcpServer(eng, t.TempDir(), embedding.NoopProvider{})
	recien, hay := valorDeGauge(t, despues.metrics.render(despues.engine), "musubi_maintenance_age_seconds")
	if !hay {
		t.Fatal("la serie desapareció tras marcar el mantenimiento")
	}
	if recien < 0 {
		t.Errorf("tras correr, la serie vale %d: sigue diciendo «nunca»", recien)
	}
	if recien > 60 {
		t.Errorf("tras correr recién, la serie vale %d segundos: no está midiendo esta corrida", recien)
	}
}

// M2 — EL CICLO NO SE DUPLICA ENTRE PROCESOS.
//
// Es lo que hace seguro que el central lo corra ADEMÁS del daemon co-residente que hoy lo corre.
// La coordinación es del DATO —`last_maintenance` en la base— y no de los procesos, así que no
// hace falta que se conozcan entre ellos. Sin esto, encender el scheduler en `serve` sería agregar
// un segundo trabajador sobre la misma base.
func TestM2ElSegundoQueLlegaNoVuelveACorrerElCiclo(t *testing.T) {
	eng := memtest.NuevoEngine(t, t.TempDir())
	// Intervalo real: con 0 el gate de arriba ni arrancaría, así que la prueba correría sobre una
	// configuración que en producción no existe.
	s := NewMcpServer(eng, t.TempDir(), embedding.NoopProvider{},
		WithMaintenance(config24h()))

	corrio, _, err := s.RunScheduledMaintenance()
	if err != nil {
		t.Fatalf("primera corrida: %v", err)
	}
	if !corrio {
		t.Fatal("la primera corrida no hizo nada: sin marca previa, el ciclo tiene que correr")
	}

	// El "segundo proceso" es otra llamada sobre la MISMA base: es exactamente lo que ve el
	// central cuando el daemon del gateway ya corrió el ciclo hace un rato.
	otra, _, err := s.RunScheduledMaintenance()
	if err != nil {
		t.Fatalf("segunda corrida: %v", err)
	}
	if otra {
		t.Error("el ciclo volvió a correr enseguida: dos procesos sobre la misma base lo harían dos veces")
	}
}

// M3 — LOS SERVIDORES QUE PUEDEN ESCRIBIR ARRANCAN EL CICLO; EL QUE NO PUEDE, NO.
//
// La guarda es de fuente, como la de la versión, y por el mismo motivo: lo que se vigila es un
// `go ...` dentro de `main`, que no tiene forma de observarse desde una prueba de paquete. Lo que
// impide que sea decoración es que las tres afirmaciones son distintas entre sí — si alguien
// borrara el scheduler de `serve`, las otras dos seguirían pasando.
//
// La tercera es la que evita el arreglo obvio y equivocado: el escalón de sólo lectura NO puede
// mantener nada —`PRAGMA query_only` rechazaría la escritura— y agregárselo «por simetría» sería
// programar trabajo que va a fallar cada vez.
func TestM3CadaServidorArrancaElCicloSegunPuedaEscribir(t *testing.T) {
	crudo, err := os.ReadFile("../../cmd/musubi/main.go")
	if err != nil {
		t.Fatalf("leer main.go: %v", err)
	}
	src := string(crudo)

	cuerpoDe := func(fn string) string {
		t.Helper()
		i := strings.Index(src, "\nfunc "+fn+"(")
		if i < 0 {
			t.Fatalf("no se encontró %s en cmd/musubi/main.go: ¿se renombró?", fn)
		}
		j := strings.Index(src[i+1:], "\n}\n")
		if j < 0 {
			t.Fatalf("no se pudo delimitar el cuerpo de %s", fn)
		}
		return src[i : i+1+j]
	}

	for _, fn := range []string{"runServe", "runDaemon"} {
		if !strings.Contains(cuerpoDe(fn), "RunMaintenanceScheduler") {
			t.Errorf("%s no arranca el ciclo de memoria. Un cerebro que no se mantiene RESPONDE IGUAL "+
				"que uno sano —la memoria sigue contestando, sólo deja de envejecer bien—, así que "+
				"la ausencia no se nota mirando. Medido: el central dependía de que el bot de "+
				"Telegram estuviera vivo.", fn)
		}
	}
	if strings.Contains(cuerpoDe("servirSoloLectura"), "RunMaintenanceScheduler") {
		t.Error("servirSoloLectura arranca el ciclo de memoria: esa base se abre con PRAGMA query_only " +
			"y toda escritura rebota, así que sería trabajo programado para fallar cada vez")
	}
}

// config24h devuelve una config de mantenimiento con el intervalo real del central.
func config24h() config.MaintenanceConfig {
	c := config.Default().Maintenance
	c.AutoIntervalHours = 24
	return c
}
