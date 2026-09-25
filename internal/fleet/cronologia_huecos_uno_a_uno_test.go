package fleet

import (
	"strings"
	"testing"
)

// CADA HUECO DE LA CRONOLOGÍA SE DECLARA UNA VEZ: ninguno falta, ninguno sobra, y ninguno se cubre
// con el texto de otro.
//
// La guarda vieja (TestLaCronologiaDeclaraLoQueNoVio) buscaba cada término obligatorio como
// SUBCADENA del texto UNIDO de todos los huecos. Eso clavaba dos ejes: DÓNDE aparece el término
// (cualquier hueco sirve) y COMO QUÉ (un pedazo de otra palabra sirve). La auditoría A131 (C3-m4)
// borró entero el hueco de los logs del host y esa guarda siguió verde: su obligatorio «log» estaba
// adentro de «cronoLOGía», en el primer hueco. El código de hoy está bien; el defecto era del
// instrumento.
//
// Ahora cada hueco de C15 (specs/flota-cronologia/spec.md) tiene una FRASE que lo nombra, y se exige
// una correspondencia UNO A UNO: cada frase está en exactamente un hueco, y cada hueco lleva
// exactamente una frase. Un hueco borrado deja su frase sin dueño; uno nuevo sin decidir queda sin
// frase; y una frase débil —«log»— se delata sola, porque aparece en dos. El texto unido ya no se
// mira: es lo que dejaba a un hueco tapar la falta de otro.
//
// EXPOSICIÓN medida por la auditoría: el estado roto no existe en producción. El binario desplegado
// lleva el hueco de los logs del host, y la tool que devuelve `no_visto` se llamó 14 veces (10 ok)
// entre el 2026-08-30 y el 2026-09-21. El agujero era de la guarda, latente.
//
// Sabotaje: borrar el hueco de los logs del host (C3-m4) → su frase queda sin hueco que la lleve.
// arnes: archivo="internal/fleet/cronologia.go"
// arnes: de="\t\t\"No hay logs del host: nada de journalctl ni del visor de eventos. Lo que se ve es lo que pasó por Musubi.\",\n"
// arnes: a=""
func TestCadaHuecoDeLaCronologiaSeDeclaraUnaSolaVez(t *testing.T) {
	// Las frases de C15, una por hueco y en minúscula: se buscan sobre cada hueco en minúscula.
	frases := []string{"serie temporal", "logs del host", "salud de servicios", "disparos de política", "contenido"}

	huecos := HuecosDeLaCronologia()
	// EL PISO: sin huecos, la correspondencia de abajo sería vacía y pasaría sin mirar nada.
	if len(huecos) == 0 {
		t.Fatal("la cronología no declara ningún hueco: una respuesta sin límites se lee como «esto es todo lo que pasó»")
	}

	enQueHuecos := map[string][]int{}
	for i, h := range huecos {
		bajo := strings.ToLower(h)
		var lleva []string
		for _, f := range frases {
			if strings.Contains(bajo, f) {
				enQueHuecos[f] = append(enQueHuecos[f], i)
				lleva = append(lleva, f)
			}
		}
		if len(lleva) != 1 {
			t.Errorf("el hueco %d lleva %d frases de C15 %q y tiene que llevar exactamente una: o es un "+
				"límite que nadie decidió, o junta dos y borrar uno se taparía con el otro\n    %s", i, len(lleva), lleva, h)
		}
	}
	for _, f := range frases {
		switch donde := enQueHuecos[f]; len(donde) {
		case 0:
			t.Errorf("ningún hueco declara %q: la respuesta dejó de decir que eso NO se miró, y una "+
				"cronología vacía ahí se lee como «no pasó nada»", f)
		case 1:
		default:
			t.Errorf("%q aparece en los huecos %v: una frase que está en dos no prueba que ninguno de los "+
				"dos exista —es la forma de «log» adentro de «cronología»—", f, donde)
		}
	}
}
