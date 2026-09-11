package fleet

// disco.go — LAS TRES COLUMNAS DEL DISCO, CALCULADAS UNA SOLA VEZ.
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// LA REGLA DE LOS PARES, Y DÓNDE FALTABA
//
// Este repo tiene una regla escrita: **un total sin su usado no se reporta**. Está dicha con esas
// palabras en el colector de macOS, para la memoria: «por la regla de los pares no se fija
// ninguno de los dos — un total sin su usado produce un 0 % que se lee como vacío».
//
// El colector de macOS la aplicaba al disco (las tres asignaciones adentro del mismo `if`) y el
// de Windows también (un `return` temprano antes de tocar nada). El de LINUX no:
//
//	m.DiscoTotal = st.Blocks * tam
//	if st.Blocks >= st.Bfree {
//	    m.DiscoUsado = (st.Blocks - st.Bfree) * tam   // ← condicional
//	}
//	m.DiscoDisponible = st.Bavail * tam               // ← incondicional
//
// Con un statfs incoherente —`Bfree > Blocks`— quedaba un TOTAL sin su USADO: el panel dibuja
// 0 % ocupado y un operador lee «disco vacío» sobre la máquina donde corre el cerebro. Y
// `DiscoDisponible` se fijaba igual, que es la única columna sobre la que alertan
// `DiscoCasiLleno` y `DiscoLleno`.
//
// Es la forma exacta del defecto dominante de este repo: la cautela escrita en dos de los tres
// hermanos, y ausente justo en el que corre en producción. Por eso la aritmética se muda acá:
// una sola regla, en un archivo SIN sufijo de plataforma, que se puede probar desde cualquier
// máquina — incluidas las dos combinaciones que en Linux sólo se dan con un filesystem roto.
// ────────────────────────────────────────────────────────────────────────────────────────────

// ColumnasDeDisco son las tres columnas de `df`: Size, Used y Avail.
//
// Son TRES y no dos porque Used + Avail ≠ Size: entre medio está la reserva de root (~5 %), que
// en Windows es el análogo de las cuotas por usuario.
type ColumnasDeDisco struct {
	Total      uint64
	Usado      uint64
	Disponible uint64
}

// ColumnasDeDiscoUnix calcula las tres desde un `statfs`. El segundo valor es `false` cuando NO
// se puede reportar el trío completo, y entonces no se reporta NADA — la regla de los pares.
func ColumnasDeDiscoUnix(blocks, bfree, bavail, bsize uint64) (ColumnasDeDisco, bool) {
	// Un `bsize` en cero convierte las tres columnas en cero: sería un «disco de 0 bytes», que
	// ninguna alerta sabe leer y todo panel dibuja como lleno.
	if blocks == 0 || bsize == 0 {
		return ColumnasDeDisco{}, false
	}
	// LA REGLA DE LOS PARES. `bfree > blocks` es un statfs incoherente; con el `if` sólo sobre el
	// usado, el total salía igual y el panel mostraba 0 % ocupado.
	if bfree > blocks || bavail > blocks {
		return ColumnasDeDisco{}, false
	}
	return ColumnasDeDisco{
		Total:      blocks * bsize,
		Usado:      (blocks - bfree) * bsize,
		Disponible: bavail * bsize,
	}, true
}

// ColumnasDeDiscoWindows calcula las tres desde `GetDiskFreeSpaceExW`.
//
// `disponibleParaElUsuario` puede ser MENOR que el libre total cuando hay cuotas activas: es el
// análogo exacto de la reserva de root en Linux, y por eso se reportan los dos números.
func ColumnasDeDiscoWindows(total, libreTotal, disponibleParaElUsuario uint64) (ColumnasDeDisco, bool) {
	if total == 0 || libreTotal > total || disponibleParaElUsuario > total {
		return ColumnasDeDisco{}, false
	}
	return ColumnasDeDisco{
		Total:      total,
		Usado:      total - libreTotal,
		Disponible: disponibleParaElUsuario,
	}, true
}
