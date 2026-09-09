package memory

// LA TRANSACCIÓN NACE ESCRITORA (X1–X3).
//
// EL DEFECTO QUE ESTO FIJA. `db.Begin()` abre una transacción DIFERIDA: empieza como lectora y se
// sube a escritora en el primer INSERT. Si entre la lectura y esa subida otra conexión escribió,
// SQLite devuelve SQLITE_BUSY_SNAPSHOT — y ese busy NO lo reintenta `busy_timeout`, vuelve al
// instante. O sea: los 5 segundos que el DSN configura no se aplican JUSTO en el caso para el que
// uno los pone.
//
// Costó siete semanas verlo. El benchmark de escala llevaba 8 corridas seguidas en rojo desde el
// 2026-07-20 con el nombre «la búsqueda vectorial dejó de escalar sublinealmente»; no era eso:
// reventaba SEMBRANDO, en la fila ~11.200, porque al cruzar las 10.000 (ExactThreshold) el propio
// engine lanza el entrenamiento del índice vectorial en segundo plano y aparece un segundo
// escritor. La guarda nunca llegó a medir lo que decía cuidar.
//
// El arreglo es `_txlock=immediate` en el DSN: `Begin()` toma el lock de escritura desde el
// principio, así no hay subida y el busy que queda SÍ es de los que `busy_timeout` espera.

import (
	"strings"
	"sync"
	"testing"
	"time"
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
	if !strings.Contains(e.path, ".musubi") {
		t.Fatalf("ruta inesperada del engine: %s", e.path)
	}
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

	tx, err := e.db.Begin()
	if err != nil {
		t.Fatalf("Begin: %v", err)
	}
	go func() {
		time.Sleep(150 * time.Millisecond)
		tx.Rollback() // suelta el lock
	}()

	inicio := time.Now()
	if err := e.SaveObservationTyped("x3", "t/x3", "una observación que espera su turno y lo consigue", 1.0, "semantic", ScopeLocal, nil); err != nil {
		t.Fatalf("la escritura no sobrevivió a la espera: %v", err)
	}
	if d := time.Since(inicio); d < 100*time.Millisecond {
		t.Errorf("la escritura tardó %v: no esperó al otro escritor, así que este caso no probó la espera", d)
	}
}

// esBaseBloqueada dice si el error es el SQLITE_BUSY que este banco existe para prohibir. Se mira
// el texto porque es lo que el driver expone; el código 5 viaja adentro del mensaje.
func esBaseBloqueada(err error) bool {
	if err == nil {
		return false
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
