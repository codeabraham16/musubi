package fleet

import (
	"fmt"
	"testing"
	"time"
)

// LA DURACIÓN SE SABE SÓLO CON LAS DOS PUNTAS Y EN ORDEN, y esta tabla recorre TODO lo que
// Duracion puede leer de un hecho.
//
// Duracion mira tres cosas y ninguna más: si `cuando` es el cero, si `termino` es el cero, y cómo se
// ordenan entre sí. Las dos primeras son binarias y la tercera tiene exactamente tres resultados en
// Go (antes, igual, después). De las doce combinaciones, cuatro no pueden existir —dos ceros sólo
// pueden ser iguales, y un cero con un no-cero nunca lo son—, así que quedan OCHO. Hay una fila por
// cada una, y el piso de abajo exige que no falte ninguna: el conjunto está cerrado por la forma de
// la comparación, no por una lista de ejemplos.
//
// LA GUARDA VIEJA (TestLaDuracionDiceSiSeSabe) CLAVABA DOS EJES, y la auditoría A131 los encontró:
//
//   - `cuando` siempre era una fecha real. Sacar el `IsZero` de `termino` (C3-m1) la dejaba verde:
//     con un `cuando` real el cero de Go queda antes y el `Before` lo descarta igual. Y tapaba algo
//     peor, vivo en el árbol sano: un `cuando` en cero —lo que deja escanearComando cuando no puede
//     leer `creado`— con `termino` puesto daba la duración saturada (~292 años) con `hay=true`.
//   - el orden se probaba antes y después, nunca IGUAL. Con `!After` en lugar de `Before` (C3-m2),
//     un hecho que terminó en el mismo segundo en que empezó —la tabla guarda segundos enteros— sale
//     «no se sabe»: `termino` con valor y `duracion_seg` en null, una fila que se contradice.
//
// LA FILA DEL `cuando` ANTERIOR AL CERO NO ES UN CAPRICHO: es la única donde el `IsZero` de
// `termino` decide algo. Con un `cuando` real el cero queda antes y el `Before` ya lo descarta; con
// uno anterior al año 1 —`time.Parse` acepta `0000-06-01T00:00:00Z`, y una fila escrita a mano puede
// traerlo— el cero queda DESPUÉS, y sin la regla explícita saldría medio año de duración para algo
// que no terminó. Sin esa fila el `IsZero` sería código muerto para toda prueba, y sacarlo pasaría
// en verde.
//
// EXPOSICIÓN medida por la auditoría en el cerebro: 30 comandos terminaron en el MISMO segundo en
// que se crearon, los 30 dentro de la ventana máxima; ninguna fila tiene `creado` ilegible.
//
// Sabotaje: sacar el `IsZero` de `termino` (C3-m1) → un hecho en curso con un comienzo anterior al
// cero reporta una duración.
// arnes: archivo="internal/fleet/cronologia.go"
// arnes: de="\tif h.Termino.IsZero() || "
// arnes: a="\tif "
// Sabotaje: descartar también el borde `termino == cuando` (C3-m2) → lo que terminó en el mismo
// segundo sale «no se sabe» con el `termino` puesto.
// arnes: archivo="internal/fleet/cronologia.go"
// arnes: de="h.Termino.Before(h.Cuando) {"
// arnes: a="!h.Termino.After(h.Cuando) {"
// Sabotaje: sacar la regla del `cuando` en cero → un hecho con fin y sin comienzo vuelve a reportar
// la duración saturada con `hay=true`, que es lo que hacía el árbol sano.
// arnes: archivo="internal/fleet/cronologia.go"
// arnes: de="\tif h.Cuando.IsZero() {\n\t\treturn 0, false\n\t}\n"
// arnes: a=""
func TestLaDuracionSeSabeSoloConLasDosPuntasEnOrden(t *testing.T) {
	inicio := time.Date(2026, 8, 25, 10, 0, 0, 0, time.UTC)
	// Un día antes del cero de Go: no es IsZero, y el cero queda DESPUÉS de él.
	antesDelCero := time.Time{}.Add(-24 * time.Hour)

	casos := []struct {
		nombre string
		h      Hecho
		hay    bool
		d      time.Duration
	}{
		{"sin ninguna de las dos puntas", Hecho{}, false, 0},
		{"sin comienzo y con fin", Hecho{Termino: inicio}, false, 0},
		{"sin comienzo y con un fin anterior al cero", Hecho{Termino: antesDelCero}, false, 0},
		{"en curso", Hecho{Cuando: inicio}, false, 0},
		{"en curso, con un comienzo anterior al cero", Hecho{Cuando: antesDelCero}, false, 0},
		{"con un fin anterior al comienzo (dato corrupto)", Hecho{Cuando: inicio, Termino: inicio.Add(-time.Minute)}, false, 0},
		{"terminado en el mismo segundo", Hecho{Cuando: inicio, Termino: inicio}, true, 0},
		{"terminado 90 s después", Hecho{Cuando: inicio, Termino: inicio.Add(90 * time.Second)}, true, 90 * time.Second},
	}

	cubiertas := map[string]bool{}
	for _, c := range casos {
		cubiertas[celdaDeDuracion(c.h)] = true
		d, hay := c.h.Duracion()
		switch {
		case hay && !c.hay:
			t.Errorf("un hecho %s reporta una duración de %s: no se sabe cuánto duró, y la cronología "+
				"dibujaría ese número como un dato", c.nombre, d)
		case !hay && c.hay:
			t.Errorf("un hecho %s dice «no se sabe» y duró %s: la fila saldría con `termino` y sin "+
				"`duracion_seg`, contradiciéndose", c.nombre, c.d)
		case hay && d != c.d:
			t.Errorf("un hecho %s reporta %s y duró %s", c.nombre, d, c.d)
		case !hay && d != 0:
			t.Errorf("un hecho %s dice «no se sabe» y aun así devuelve %s: el número viaja aunque el "+
				"booleano diga que no", c.nombre, d)
		}
	}

	// EL PISO: cada combinación posible de lo que Duracion lee tiene su fila. Sin esto, una tabla a
	// la que se le borra una fila seguiría verde midiendo menos de lo que dice.
	for _, celda := range celdasDeDuracionPosibles() {
		if !cubiertas[celda] {
			t.Errorf("ninguna fila cubre la combinación %s: la tabla dejó un hueco en el dominio", celda)
		}
	}
	if n := len(celdasDeDuracionPosibles()); n != 8 {
		t.Fatalf("las combinaciones posibles son %d y tienen que ser 8: el piso está mal armado", n)
	}
}

// celdaDeDuracion nombra la combinación de lo que Duracion lee de un hecho: cada punta en cero o
// no, y el orden entre las dos.
func celdaDeDuracion(h Hecho) string {
	return fmt.Sprintf("cuando_cero=%v termino_cero=%v orden=%+d", h.Cuando.IsZero(), h.Termino.IsZero(), h.Termino.Compare(h.Cuando))
}

// celdasDeDuracionPosibles son las combinaciones que pueden existir: dos ceros son iguales, y un
// cero y un no-cero no lo son nunca.
func celdasDeDuracionPosibles() []string {
	var out []string
	for _, cuandoCero := range []bool{true, false} {
		for _, terminoCero := range []bool{true, false} {
			for _, orden := range []int{-1, 0, 1} {
				if cuandoCero && terminoCero && orden != 0 {
					continue
				}
				if cuandoCero != terminoCero && orden == 0 {
					continue
				}
				out = append(out, fmt.Sprintf("cuando_cero=%v termino_cero=%v orden=%+d", cuandoCero, terminoCero, orden))
			}
		}
	}
	return out
}
