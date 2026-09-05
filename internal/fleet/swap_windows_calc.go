package fleet

// swap_windows_calc.go — la aritmética del swap de Windows, FUERA del build tag.
//
// ════════════════════════════════════════════════════════════════════════════════════════════
// VIVE ACÁ Y NO EN colector_windows.go PARA PODER PROBARLA DESDE CUALQUIER MÁQUINA
//
// Es el mismo criterio que los parsers de servicios, y por la misma lección: lo que sólo compila
// bajo `//go:build windows` no lo prueba nadie desde el Linux donde se desarrolla, y el defecto
// que esto arregla vivió justamente ahí — en cuatro líneas de resta que ningún test tocaba.

// SwapDeWindows deriva el swap real a partir de los cuatro contadores de MEMORYSTATUSEX.
//
// El «page file» de Windows es lo más cercano al swap, pero `totalPagina` INCLUYE la RAM, así que
// el swap real es la diferencia entre el commit total y la memoria física. Lo mismo del lado
// disponible.
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// EL BUG: NO PODER CALCULAR LO LIBRE SE REPORTABA COMO «SWAP 100 % LLENO»
//
// La versión anterior arrancaba `libre := 0` y sólo lo pisaba si `disponiblePagina >
// disponibleFisica`. Cuando esa resta no daba —una máquina con mucha RAM libre y un page file
// chico, que es una máquina SANA— el cero se quedaba puesto y salía `SwapUsada == SwapTotal`.
//
// Eso no es un matiz perdido: es el colector AFIRMANDO que el swap está lleno porque no supo
// medirlo. Es la regla del cero mentiroso, y del lado del que más caro sale — el que inventa una
// emergencia. La misma forma que A70 (`ocioso` exportado como caído) y que el `desconocido` del
// exportador de servicios.
//
// Ahora los dos números salen JUNTOS o no sale ninguno, que es la regla de los pares que muestra.go
// ya aplica a memoria y a disco. Un swap que no se pudo medir viaja como «esta máquina no reporta
// swap», que es la verdad.
func SwapDeWindows(totalPagina, totalFisica, disponiblePagina, disponibleFisica uint64) (total, usada uint64, ok bool) {
	// Sin page file más allá de la RAM no hay swap del que hablar. No es un fallo de medición:
	// es una máquina sin swap, y eso ya se dice no reportando el par.
	if totalPagina <= totalFisica {
		return 0, 0, false
	}
	total = totalPagina - totalFisica
	// SI LO DISPONIBLE NO SE PUEDE DESPEJAR, NO SE INVENTA. `disponiblePagina` es el commit
	// disponible y `disponibleFisica` la RAM disponible; que la resta no dé positiva es normal en
	// una máquina con mucha RAM libre y un page file chico. Lo único que se sabe entonces es que
	// no se sabe.
	if disponiblePagina <= disponibleFisica {
		return 0, 0, false
	}
	libre := disponiblePagina - disponibleFisica
	if libre > total {
		// Incoherente entre sí: el swap libre no puede superar al total. Se descarta el par
		// entero en vez de recortarlo, igual que hace Muestra.Valida con una muestra que no cierra.
		return 0, 0, false
	}
	return total, total - libre, true
}
