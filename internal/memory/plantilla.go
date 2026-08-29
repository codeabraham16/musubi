package memory

// plantilla.go es SOPORTE DE PRUEBAS, y no lo llama ningún camino de producción.
//
// ════════════════════════════════════════════════════════════════════════════════════════════
// POR QUÉ EXISTE (A45)
//
// Cada prueba que necesita una base abría una nueva y aplicaba las 39 migraciones de cero. Sin
// `-race` eso cuesta décimas y nadie lo mira. Bajo `-race`, `modernc.org/sqlite` —el SQLite en Go
// puro que este proyecto usa para no tener cgo— corre ~10× más lento y el costo se vuelve el
// presupuesto entero. MEDIDO en este árbol: 15 pruebas de `internal/mcp` bajo `-race` tardaban
// 69,8 s migrando de cero y tardan 12,9 s con plantilla. Son 280 pruebas en `internal/mcp` y 82
// en `internal/memory`: por eso la CI, que sí usa `-race`, nunca entraba en su `-timeout 30m`.
//
// Y EMPEORABA SOLA: cada migración nueva le sumaba a las 362 aperturas a la vez.
//
// ════════════════════════════════════════════════════════════════════════════════════════════
// EL ARREGLO NO AGREGA UNA SOLA RAMA A NewDbEngine
//
// Las migraciones son deterministas: sobre una base vacía dan siempre el mismo archivo. Se pagan
// UNA VEZ por binario de prueba y después se copia el resultado. La prueba siembra el archivo
// ANTES de llamar a NewDbEngine, y su runMigrations lee `user_version` = la última y no hace nada.
//
// Esa es la propiedad que importa: una optimización de pruebas que metiera una rama adentro de
// NewDbEngine compraría velocidad con el riesgo de que el camino que corre en la CI no sea el que
// corre en el servidor. Acá el código de producción es EL MISMO en los dos lados.
//
// ════════════════════════════════════════════════════════════════════════════════════════════
// POR QUÉ VIVE EN ESTE PAQUETE Y NO EN internal/memory/memtest
//
// Las pruebas de `internal/memory` son del paquete `memory`, así que importar `memtest` —que
// importa `memory`— sería un ciclo. El núcleo vive acá y `memtest` es un envoltorio fino con
// `*testing.T` para los paquetes de afuera. La alternativa era duplicar treinta líneas en dos
// lados, que es exactamente cómo una de las dos copias se queda vieja.

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	"musubi/internal/config"
)

var (
	plantillaUnaVez sync.Once
	plantillaRuta   string
	plantillaErr    error
)

// SembrarPlantillaDePruebas deja en projectPath una base YA MIGRADA, lista para que NewDbEngine
// la abra sin aplicar una sola migración. SÓLO PARA PRUEBAS.
//
// NO SE DEGRADA EN SILENCIO. Si la plantilla no se puede construir devuelve el error en vez de
// dejar que la prueba migre de cero: una caída silenciosa dejaría A45 abierto con la suite en
// verde y nadie volvería a mirarlo — que es el modo de fallo que este árbol persigue.
func SembrarPlantillaDePruebas(projectPath string) error {
	plantillaUnaVez.Do(construirPlantillaDePruebas)
	if plantillaErr != nil {
		return plantillaErr
	}
	destino := filepath.Join(projectPath, config.DirName, config.DBFile)
	if err := os.MkdirAll(filepath.Dir(destino), 0o755); err != nil {
		return err
	}
	return copiarArchivo(plantillaRuta, destino)
}

// construirPlantillaDePruebas corre UNA VEZ por binario de prueba.
//
// El directorio no se borra: no hay un gancho de salida de proceso en Go que no sea un TestMain
// por paquete, y agregar uno a cada paquete para limpiar 434 KB sería más código del que vale.
// Queda con un nombre reconocible —`musubi-plantilla-esquema-*` en el temp del sistema— y lo
// levanta el limpiador del sistema operativo como cualquier otro temporal.
//
// El `sync.Once` es lo que hace segura la concurrencia: las pruebas con `t.Parallel()` pueden
// llamar a la vez, la construcción se serializa, y cada una copia SU archivo desde la plantilla
// ya lista. Nadie escribe sobre el original.
func construirPlantillaDePruebas() {
	dir, err := os.MkdirTemp("", "musubi-plantilla-esquema-*")
	if err != nil {
		plantillaErr = err
		return
	}
	eng, err := NewDbEngine(dir)
	if err != nil {
		plantillaErr = fmt.Errorf("no se pudo migrar la plantilla: %w", err)
		return
	}
	ruta := filepath.Join(dir, config.DirName, config.DBFile)

	// CERRAR ANTES DE REGISTRAR LA PLANTILLA ES LA LÍNEA LOAD-BEARING DE ESTE ARCHIVO.
	//
	// La base corre en WAL: con el engine ABIERTO, `memory.db` está prácticamente vacío y el
	// esquema entero vive en `memory.db-wal`. Medido en este árbol, recién migrada:
	//
	//     abierto   →  memory.db = 4.096 bytes,  memory.db-wal = 910.552 bytes
	//     cerrado   →  memory.db = 434.176 bytes, el -wal ya no existe
	//
	// Copiar el archivo principal sin cerrar daría una plantilla de 4 KB con `user_version` en 0,
	// y entonces CADA prueba volvería a migrar de cero SIN QUE NADA LO DIGA: la suite quedaría
	// igual de lenta y en VERDE, con A45 abierto y nadie mirándolo otra vez.
	//
	// NO HACE FALTA UN `wal_checkpoint` EXPLÍCITO, y conviene escribir por qué en vez de dejar la
	// llamada «por las dudas»: SQLite hace checkpoint solo al cerrar la ÚLTIMA conexión, y
	// `db.Close()` cierra todas las del pool. Se midió antes de sacarlo — los números de arriba
	// son de esa medición. Lo que hay que cuidar es el ORDEN, y eso lo custodia
	// TestLaPlantillaLlegaConElEsquemaAlDia, que mira el `user_version` del archivo copiado sin
	// importarle por qué mecanismo llegó ahí.
	eng.Close()

	info, err := os.Stat(ruta)
	if err != nil || info.Size() == 0 {
		plantillaErr = fmt.Errorf("la plantilla quedó vacía o ilegible: %v", err)
		return
	}
	plantillaRuta = ruta
}

func copiarArchivo(desde, hasta string) error {
	in, err := os.Open(desde)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(hasta)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}
