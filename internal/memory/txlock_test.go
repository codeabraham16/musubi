package memory

// LA TRANSACCIÓN NACE ESCRITORA (X1–X3).
//
// EL DEFECTO QUE ESTO FIJA. `db.Begin()` abre una transacción DIFERIDA: empieza como lectora y se
// sube a escritora en el primer INSERT. Si entre la lectura y esa subida otra conexión escribió,
// SQLite devuelve SQLITE_BUSY_SNAPSHOT — y ese busy NO lo reintenta `busy_timeout`, vuelve al
// instante. O sea: los 5 segundos que el DSN configura no se aplican JUSTO en el caso para el que
// uno los pone.
//
// Costó siete semanas verlo. El benchmark de escala llevaba 8 corridas y las 8 rojas —toda su
// historia desde el 2026-07-20, nunca una verde— bajo el nombre «la búsqueda vectorial dejó de
// escalar sublinealmente»; no era eso:
// reventaba SEMBRANDO, en la fila ~11.200, porque al cruzar las 10.000 (ExactThreshold) el propio
// engine lanza el entrenamiento del índice vectorial en segundo plano y aparece un segundo
// escritor. La guarda nunca llegó a medir lo que decía cuidar.
//
// El arreglo es `_txlock=immediate` en el DSN: `Begin()` toma el lock de escritura desde el
// principio, así no hay subida y el busy que queda SÍ es de los que `busy_timeout` espera.

import (
	"errors"
	"math/rand"
	"strings"
	"sync"
	"testing"
	"time"

	sqlite "modernc.org/sqlite"
)

// X1 — DOS ESCRITORES CONCURRENTES NO SE MATAN.
//
// Es la reproducción chica de lo que le pasaba al sembrado: un lado abre transacción, LEE, y recién
// después escribe (el patrón de saveObservation), mientras el otro escribe sin parar. Con la
// transacción diferida, la subida de lector a escritor revienta con SQLITE_BUSY inmediato.
//
// Se afirma sobre el ERROR, no sobre un contador: lo que se quiere prohibir es exactamente
// «database is locked», no cualquier fallo.
func TestX1DosEscritoresConcurrentesNoSeMatan(t *testing.T) {
	e := nuevoEngineDePrueba(t)

	// Semilla para que la lectura de la transacción larga tenga algo que leer.
	for i := 0; i < 20; i++ {
		if err := e.SaveObservationTyped(idDePrueba("semilla", i), "t/x1", "una nota de semilla suficientemente larga", 1.0, "semantic", ScopeLocal, nil); err != nil {
			t.Fatalf("sembrar: %v", err)
		}
	}

	var wg sync.WaitGroup
	errs := make(chan error, 64)

	// Lado A: el patrón leer-y-después-escribir, sostenido en el tiempo.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 12; i++ {
			tx, err := e.db.Begin()
			if err != nil {
				errs <- err
				return
			}
			var n int
			if err := tx.QueryRow(`SELECT COUNT(*) FROM observations`).Scan(&n); err != nil {
				tx.Rollback()
				errs <- err
				return
			}
			time.Sleep(2 * time.Millisecond) // deja lugar a que el otro lado escriba en el medio
			if _, err := tx.Exec(`UPDATE observations SET importance = importance WHERE id = ?`, idDePrueba("semilla", 0)); err != nil {
				tx.Rollback()
				errs <- err
				return
			}
			if err := tx.Commit(); err != nil {
				errs <- err
				return
			}
		}
	}()

	// Lado B: escrituras normales por el camino público, que es como entra la memoria de verdad.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 40; i++ {
			if err := e.SaveObservationTyped(idDePrueba("x1", i), "t/x1", "una observación cualquiera, suficientemente larga para pasar", 1.0, "semantic", ScopeLocal, nil); err != nil {
				errs <- err
				return
			}
		}
	}()

	wg.Wait()
	close(errs)
	for err := range errs {
		if esBaseBloqueada(err) {
			// El mensaje NO afirma cuál de las dos mitades falló, porque este caso no lo mide:
			// hay busy tanto si `Begin()` nace diferida (y la subida de lector a escritor vuelve al
			// instante) como si nace escritora pero no hay espera configurada. X2 separa la primera;
			// X3, la segunda. Nombrar una sola sería una causa inventada — y ya costó siete semanas
			// creerle a un rótulo que no había medido nada.
			t.Fatalf("SQLITE_BUSY con dos escritores: al DSN le falta `_txlock=immediate` (la transacción nace DIFERIDA y la subida de lector a escritor no espera) o le falta el `busy_timeout` (no hay espera que esperar) — %v", err)
		}
		t.Fatalf("error inesperado: %v", err)
	}
}

// X2 — Y EL DSN LO DICE.
//
// X1 puede pasar por suerte: con poca contención la subida a veces no choca. Este caso fija la
// CAUSA, no el síntoma, y es el que se rompe de forma determinista si alguien saca el pragma del
// DSN sin querer.
func TestX2ElDSNAbreLasTransaccionesComoEscritoras(t *testing.T) {
	e := nuevoEngineDePrueba(t)
	// (Acá había un `strings.Contains(e.path, ".musubi")` que no podía fallar: la ruta la arma el
	// propio helper. Una aserción que no puede ponerse roja no verifica nada y hace parecer que el
	// caso cubre más de lo que cubre.)
	//
	// La verificación va contra el comportamiento observable, no contra el texto del DSN: se abre
	// una transacción y se comprueba que YA tiene el lock de escritura, o sea que una segunda
	// transacción escritora no puede tomarlo al mismo tiempo.
	tx1, err := e.db.Begin()
	if err != nil {
		t.Fatalf("Begin 1: %v", err)
	}
	defer tx1.Rollback()

	hecho := make(chan error, 1)
	go func() {
		tx2, err := e.db.Begin()
		if err != nil {
			hecho <- err
			return
		}
		tx2.Rollback()
		hecho <- nil
	}()

	select {
	case err := <-hecho:
		if err == nil {
			t.Error("dos transacciones tomaron el lock de escritura a la vez: `Begin()` está naciendo DIFERIDA y la exclusión recién ocurriría en el primer INSERT, que es tarde")
		}
	case <-time.After(1500 * time.Millisecond):
		// Esperar es el comportamiento CORRECTO: la segunda transacción se queda en el
		// busy_timeout hasta que la primera suelte. Que espere prueba que la primera ya es
		// escritora desde el Begin.
	}
}

// X3 — Y LA ESPERA TERMINA: no se cambió un fallo por un cuelgue.
//
// `_txlock=immediate` convierte un error inmediato en una espera acotada, y una espera acotada sólo
// sirve si de verdad termina. Sin este caso, «arreglar» el busy poniendo un lock global que nadie
// suelta pasaría X1 y X2 con las mejores notas.
func TestX3LaEsperaTerminaCuandoElOtroSuelta(t *testing.T) {
	e := nuevoEngineDePrueba(t)

	// EL PISO SE CALIBRA, NO SE CLAVA. La versión anterior comparaba contra 100 ms fijos, y esa
	// constante tiene una asimetría fea: el caso nunca se pone falsamente ROJO (con el DSN sano la
	// espera es ≥ la retención), pero se pone falsamente VERDE en cuanto una escritura SIN
	// contención cruza los 100 ms por lentitud de la máquina — y ahí el sabotaje que saca
	// `_txlock=immediate` pasaría con las mejores notas, callado. No es hipotético: bajo `-race`
	// el driver corre mucho más lento (ci.yml:46-58 documenta la corrida completa en 8m12s con
	// -race contra la de sin), y `test` de CI corre justamente con -race.
	//
	// Así que primero se mide cuánto tarda una escritura sin nadie enfrente, EN ESTA máquina y en
	// ESTE momento, y el piso se arma a partir de ese número. Una escritura de calentamiento
	// antes, para no medir la inicialización perezosa del pool.
	if err := e.SaveObservationTyped("x3-calentamiento", "t/x3", "una escritura de calentamiento, para no medir la inicialización perezosa", 1.0, "semantic", ScopeLocal, nil); err != nil {
		t.Fatalf("calentamiento: %v", err)
	}
	inicioLibre := time.Now()
	if err := e.SaveObservationTyped("x3-calibracion", "t/x3", "la escritura de calibración: cuánto tarda esto cuando no hay nadie enfrente", 1.0, "semantic", ScopeLocal, nil); err != nil {
		t.Fatalf("calibración: %v", err)
	}
	libre := time.Since(inicioLibre)

	// La retención tiene que ser cómodamente menor que el busy_timeout(5000) para que la escritura
	// llegue a conseguir su turno, y cómodamente mayor que el jitter de la máquina.
	const retencion = 400 * time.Millisecond

	tx, err := e.db.Begin()
	if err != nil {
		t.Fatalf("Begin: %v", err)
	}
	go func() {
		time.Sleep(retencion)
		tx.Rollback() // suelta el lock
	}()

	inicio := time.Now()
	if err := e.SaveObservationTyped("x3", "t/x3", "una observación que espera su turno y lo consigue", 1.0, "semantic", ScopeLocal, nil); err != nil {
		t.Fatalf("la escritura no sobrevivió a la espera: %v", err)
	}
	esperado := time.Since(inicio)

	// Si esperó, tarda ≈ retención + libre. Si NO esperó, tarda ≈ libre. El piso parte la
	// diferencia, así que escala solo con la lentitud de la máquina en vez de quedarse fijo.
	piso := libre + retencion/2
	if esperado < piso {
		t.Errorf("la escritura tardó %v y el piso calibrado era %v (escritura libre: %v, retención: %v): "+
			"no esperó al otro escritor, así que este caso no probó la espera",
			esperado, piso, libre, retencion)
	}
}

// X4 — EL ENTRENADOR DE FONDO REAL NO MATA AL SEMBRADO.
//
// X1–X3 miden la MECÁNICA del lock, y la miden bien, pero fabrican el segundo escritor a mano con
// un `e.db.Begin()` adentro del test. El escenario que de verdad rompió no era ése: al cruzar
// `ExactThreshold` el propio engine lanza el entrenamiento del índice vectorial en segundo plano
// (maybeRebuildVectorIndex → spawnBackground → rebuild + snapshot + MarkMetaNow) y aparece un
// segundo escritor QUE NADIE DECLARÓ. Sin este caso, ese camino sólo lo ejercita `bench-scale`:
// semanal, opt-in y a 100.000 filas — o sea, la regresión volvería a tardar semanas en aparecer.
//
// La diferencia con TestProactiveTrainAcrossThreshold, que también cruza el umbral, es de una
// línea: aquél llama `e.bgWG.Wait()` inmediatamente después de cruzarlo, o sea que deja de
// escribir justo cuando empezaría la contención. Acá se sigue escribiendo MIENTRAS el entrenador
// corre, que es la única forma de que los dos escritores se encuentren.
func TestX4ElEntrenadorDeFondoNoMataAlSembrado(t *testing.T) {
	e, err := NewDbEngine(dirSembrado(t))
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close()
	e.bgWG.Wait() // settle del autobuild de arranque

	const dim = 16
	e.vindexCfg.ExactThreshold = 8
	rng := rand.New(rand.NewSource(7))

	// Justo debajo del umbral: la próxima alta lo cruza y dispara el entrenamiento de fondo.
	e.index.seedDirty(e.vindexCfg.ExactThreshold - 1)

	if err := e.SaveObservation("x4-cruce", "t/x4", "la observación que cruza el umbral y despierta al entrenador", randomVec(rng, dim)); err != nil {
		t.Fatalf("la escritura que cruza el umbral falló: %v", err)
	}

	// SIN bgWG.Wait(): se sigue escribiendo mientras el entrenador trabaja. Es acá donde antes
	// aparecía `database is locked`, porque saveObservation lee y después escribe en la misma
	// transacción y el entrenador ya había escrito en el medio.
	for i := 0; i < 60; i++ {
		err := e.SaveObservation(idDePrueba("x4", i), "t/x4", "una observación más, escrita mientras el índice se entrena en segundo plano", randomVec(rng, dim))
		if esBaseBloqueada(err) {
			t.Fatalf("SQLITE_BUSY en la escritura %d mientras el entrenador de fondo corría: es exactamente el fallo que reventaba el sembrado de bench-scale en la fila ~11.200 — %v", i, err)
		}
		if err != nil {
			t.Fatalf("error inesperado en la escritura %d: %v", i, err)
		}
	}

	e.bgWG.Wait()
}

// X5 — CONSOLIDAR NO SE QUEDA CON EL LOCK DURANTE EL BARRIDO.
//
// `_txlock=immediate` corre la ventana de exclusión hacia atrás, hasta el `Begin`. Eso convierte
// en un problema algo que antes era gratis: abrir la transacción al principio de la función y
// recién escribir mucho después. `Consolidate` hacía justo eso —abría antes del emparejamiento por
// trigramas, que es CPU pura y no toca la base—, así que con el pragma puesto pasaba a sostener el
// lock durante todo el barrido: medido, 8,9 s a 40.000 observaciones, más que el
// `busy_timeout(5000)`, y los demás escritores empezaban a fallar.
//
// El caso es DETERMINISTA a propósito, sin cronómetro: se toma el lock de escritura y se deja
// tomado. Si `Consolidate` no tiene nada que escribir, no debe pedirlo, y termina igual. Con el
// código anterior se quedaba esperando los 5 s del busy_timeout y moría.
func TestX5ConsolidarNoTomaElLockSiNoTieneQueEscribir(t *testing.T) {
	e := nuevoEngineDePrueba(t)

	// Sembrar observaciones bien distintas entre sí: el régimen de una base YA consolidada, que es
	// el estado normal y el que más veces se recorre. El vocabulario tiene que ser DISTINTO de
	// verdad, no sólo numerado: una plantilla común con un número que cambia comparte casi todos
	// los trigramas y el emparejamiento la fusiona igual (así falló la primera versión de este
	// caso, y el rojo se leía como una regresión del arreglo).
	sujetos := []string{"el compilador", "la bicicleta", "un volcán", "la cosecha", "el submarino", "la partitura", "un telescopio", "la marea"}
	verbos := []string{"cruje sin aviso", "florece temprano", "se hunde despacio", "acelera de golpe", "descansa quieto"}
	lugares := []string{"en el puerto viejo", "bajo la nieve fina", "cerca del faro roto", "dentro del túnel largo", "sobre la cornisa húmeda", "junto al río turbio", "en la bodega fría", "tras la colina seca"}
	const n = 40
	for i := 0; i < n; i++ {
		contenido := sujetos[i%len(sujetos)] + " " + verbos[(i/len(sujetos))%len(verbos)] + " " + lugares[(i*3)%len(lugares)] + ", según la anotación " + itoaCorto(i) + "."
		if err := e.SaveObservationTyped(idDePrueba("x5", i), "t/x5", contenido, 1.0, "semantic", ScopeLocal, nil); err != nil {
			t.Fatalf("sembrar %d: %v", i, err)
		}
	}

	// PRIMERO SIN EL LOCK, para separar los dos modos de falla. Si la semilla tuviera duplicados,
	// `Consolidate` necesitaría escribir y el caso de abajo fallaría por SQLITE_BUSY — un rojo que
	// se leería como «el arreglo se rompió» cuando en realidad la culpa sería de la semilla. Así
	// cada falla dice lo suyo.
	previo, err := e.Consolidate(0.9)
	if err != nil {
		t.Fatalf("la consolidación de control falló: %v", err)
	}
	if previo.Merged != 0 {
		t.Fatalf("la semilla tenía %d duplicados: el caso de abajo no probaría lo que dice probar", previo.Merged)
	}
	if previo.Scanned != n {
		t.Fatalf("escaneó %d observaciones y se sembraron %d: el barrido no recorrió lo que se cree", previo.Scanned, n)
	}

	// Y AHORA CON EL LOCK TOMADO. Con `_txlock=immediate` esta transacción es dueña del lock de
	// escritura desde el Begin, y no lo suelta en todo el caso.
	tx, err := e.db.Begin()
	if err != nil {
		t.Fatalf("Begin: %v", err)
	}
	defer tx.Rollback()

	res, err := e.Consolidate(0.9)
	if err != nil {
		t.Fatalf("Consolidate pidió el lock de escritura sin tener nada que escribir (la corrida de "+
			"control ya midió 0 fusiones sobre esta misma semilla): está abriendo la transacción ANTES "+
			"del barrido por trigramas, que es CPU pura — %v", err)
	}
	if res.Merged != 0 {
		t.Fatalf("fusionó %d con el lock tomado, cuando la corrida de control fusionó 0", res.Merged)
	}
}

// esBaseBloqueada dice si el error es el SQLITE_BUSY que este banco existe para prohibir.
//
// PRIMERO EL CÓDIGO, DESPUÉS EL TEXTO, y las dos cosas a propósito. El driver expone el código
// tipado (modernc.org/sqlite/error.go:12-21: `type Error struct` con `Code() int`), así que
// clasificar por el mensaje sería frágil sin necesidad: alcanza con que una versión reescriba la
// cadena para que este banco deje de ver lo que vino a vigilar —callándose, que es el peor modo—.
// SQLITE_BUSY es el código primario 5; los extendidos (SQLITE_BUSY_SNAPSHOT = 5|2<<8 = 517) lo
// llevan en el byte bajo, por eso el `&0xFF`.
//
// El texto queda como RED, no como método principal: un error que ya cruzó una capa que lo
// convirtió en cadena (un `errors.New(err.Error())` en el camino) pierde el tipo, y en ese caso
// preferimos detectarlo igual a dejarlo pasar.
func esBaseBloqueada(err error) bool {
	if err == nil {
		return false
	}
	var errSQLite *sqlite.Error
	if errors.As(err, &errSQLite) && errSQLite.Code()&0xFF == 5 {
		return true
	}
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "database is locked") || strings.Contains(s, "sqlite_busy")
}

func idDePrueba(pre string, i int) string {
	return pre + "-" + string(rune('a'+i%26)) + "-" + itoaCorto(i)
}

func itoaCorto(i int) string {
	if i == 0 {
		return "0"
	}
	var b []byte
	for i > 0 {
		b = append([]byte{byte('0' + i%10)}, b...)
		i /= 10
	}
	return string(b)
}
