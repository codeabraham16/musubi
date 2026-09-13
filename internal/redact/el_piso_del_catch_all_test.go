package redact

import (
	"math"
	"os"
	"strings"
	"testing"
)

// leerFuente lee un archivo de ESTE paquete. La guarda de abajo pregunta por el COMENTARIO de la
// calibración, así que necesita el fuente y no los valores compilados.
func leerFuente(t *testing.T, nombre string) string {
	t.Helper()
	b, err := os.ReadFile(nombre)
	if err != nil {
		t.Fatalf("no pude leer %s: %v — no medí nada", nombre, err)
	}
	return string(b)
}

// TestElCatchAllDeEntropiaTieneUnPisoYEstaDeclarado fija el límite matemático del catch-all y
// obliga a que esté escrito al lado de la calibración.
//
// ────────────────────────────────────────────────────────────────────────────────────────────────
// QUÉ SE DESCUBRIÓ, Y CÓMO
//
// El comentario de la calibración dice que el umbral «pega en secretos aleatorios (entropía
// ~5.5-6)». Es cierto para los LARGOS y es imposible para los cortos, y la razón no es empírica:
//
//	la entropía de Shannon de una cadena de n caracteres no puede pasar de log2(n)
//
// porque con n caracteres hay a lo sumo n símbolos distintos, y el máximo se alcanza cuando todos
// son distintos: H = log2(n). Para llegar a 4,5 hacen falta n ≥ 2^4,5 ≈ 22,63, o sea 23 caracteres.
//
// CONSECUENCIA: TODO SECRETO DE 22 CARACTERES O MENOS ES INVISIBLE PARA ESTE CATCH-ALL, por más
// aleatorio que sea. No es una calibración floja que se pueda apretar bajando el umbral: es el techo
// de la métrica. Un secreto de 16 caracteres con 91 bits de entropía real —criptográficamente
// sobrado— tiene entropía de Shannon ≤ 4,0 y no lo ve nunca.
//
// ASÍ SE ENCONTRÓ, y vale porque explica por qué nadie lo había visto: `NuevaPassPantalla`
// (internal/fleet) acuña 16 caracteres y atraviesa el redactor por 6 de 9 formas. Buscando por qué,
// el número salió solo. El defecto no estaba en la pass: estaba en creer que el catch-all cubre lo
// que no puede cubrir.
//
// POR QUÉ ESTA GUARDA NO «ARREGLA» NADA. Bajar el umbral para que entren los cortos llenaría de
// [REDACTED] cualquier prosa; subir `minTokenLen` no cambia el techo. Lo que hay que evitar es la
// CREENCIA FALSA: que alguien acuñe una credencial corta pensando que el catch-all la cubre. Por eso
// lo que se fija es que el piso esté DECLARADO donde se calibra — un número que nadie escribió es un
// número que nadie va a recordar.
//
// Sabotaje que la hace fallar: en `redact.go`, subir `entropyThreshold` sin mover el piso declarado
// —por ejemplo a 5.0— y el piso pasa de 23 a 33 caracteres sin que el comentario lo diga.
// arnes: archivo="internal/redact/redact.go"
// arnes: de="\tentropyThreshold = 4.5"
// arnes: a="\tentropyThreshold = 5.0"
// arnes: prueba="TestElCatchAllDeEntropiaTieneUnPisoYEstaDeclarado"
func TestElCatchAllDeEntropiaTieneUnPisoYEstaDeclarado(t *testing.T) {
	// EL PISO, DERIVADO DE LA CONSTANTE Y NO ESCRITO A MANO. Si se escribiera el 23, la guarda
	// quedaría midiendo la calibración de ayer en cuanto alguien mueva el umbral.
	piso := int(math.Ceil(math.Pow(2, entropyThreshold)))

	t.Run("el piso es real: ni la cadena de MAXIMA entropia lo alcanza por debajo", func(t *testing.T) {
		// Se construye el peor caso PARA LA GUARDA: todos los caracteres distintos, que es la
		// entropía máxima posible a ese largo. Si ni ése pasa, ninguno pasa.
		alfabeto := "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789_-"
		for n := minTokenLen; n < piso; n++ {
			s := alfabeto[:n]
			if h := shannonEntropy(s); h >= entropyThreshold {
				t.Fatalf("una cadena de %d caracteres TODOS DISTINTOS dio entropía %.4f ≥ %.1f: "+
					"el piso derivado (%d) está mal y toda esta guarda mide otra cosa",
					n, h, entropyThreshold, piso)
			}
		}
		// Y LA OTRA DIRECCIÓN, que es la que impide que esto sea un «siempre pasa»: en el piso
		// justo, la cadena de máxima entropía SÍ tiene que alcanzarlo. Sin esto, un piso absurdo
		// —digamos 10.000— también haría pasar el bucle de arriba.
		if piso <= len(alfabeto) {
			s := alfabeto[:piso]
			if h := shannonEntropy(s); h < entropyThreshold {
				t.Errorf("en el piso justo (%d caracteres distintos) la entropía es %.4f < %.1f: "+
					"entonces el piso no es %d y el número declarado miente",
					piso, h, entropyThreshold, piso)
			}
		}
	})

	t.Run("un secreto de 16 caracteres atraviesa el catch-all, y eso NO es un bug del catch-all", func(t *testing.T) {
		// El caso concreto que lo destapó, fijado para que nadie lo lea como una falla a arreglar
		// bajando el umbral. 16 caracteres de un alfabeto de 52: ~91 bits de entropía REAL.
		corto := "apqrstuvwxyzACDE" // 16, todos distintos: el mejor caso posible a ese largo
		if len(corto) >= piso {
			t.Fatalf("el caso de prueba tiene %d caracteres y el piso es %d: dejó de ser un caso "+
				"por debajo del piso y esta rama no mide nada", len(corto), piso)
		}
		salida, _ := Redact("pass: " + corto)
		if !strings.Contains(salida, corto) {
			t.Errorf("un secreto de %d caracteres quedó tapado: %s\n"+
				"  Si ahora lo tapa, es por una REGLA POR FORMA nueva, no por el catch-all de "+
				"entropía —que no puede—. Revisá que la regla cubra el caso a propósito y actualizá "+
				"el piso declarado.", len(corto), salida)
		}
	})

	t.Run("el piso esta DECLARADO en el comentario de la calibracion", func(t *testing.T) {
		// LA MITAD QUE IMPORTA DE VERDAD: el agujero no se cierra, se DICE. Un catch-all cuyo límite
		// no está escrito al lado invita a confiar en él para lo que no puede hacer, que es
		// exactamente cómo nació la credencial de 16 caracteres.
		fuente := leerFuente(t, "redact.go")
		i := strings.Index(fuente, "entropyThreshold = ")
		if i < 0 {
			t.Fatal("no encontré la calibración en redact.go: no medí nada")
		}
		cabecera := fuente[:i]

		// LOS NÚMEROS SE DERIVAN Y SE EXIGEN; NO SE PREGUNTA POR UN TEXTO FIJO.
		//
		// LA PRIMERA VERSIÓN DE ESTA RAMA ERA HUECA Y LO DIJO SU PROPIO SABOTAJE. Preguntaba si el
		// comentario contenía los literales «22» y «23». Como el piso se deriva de la constante,
		// subir el umbral movía el piso Y dejaba el comentario intacto: los literales seguían ahí y
		// la guarda seguía verde. Era la pregunta tautológica — derivé lo mismo que tenía que
		// controlar. Medido: con `entropyThreshold = 5.0` el arnés la reportó EN VERDE sobre su
		// propio sabotaje.
		//
		// Ahora se exige el número que el piso derivado IMPLICA. Con 4.5 el comentario tiene que
		// hablar de 22 y 23; si alguien lo sube a 5.0, tiene que hablar de 31 y 32, y si no lo
		// actualizó, esto se pone rojo. Eso es lo que hace que el comentario no pueda quedar rancio.
		ultimoInvisible := itoa(piso - 1)
		primeroVisible := itoa(piso)
		for _, aguja := range []string{"log2", ultimoInvisible, primeroVisible} {
			if !strings.Contains(cabecera, aguja) {
				t.Errorf("el comentario de la calibración no menciona %q.\n"+
					"  Con entropyThreshold = %.2f el piso derivado es %d: el catch-all NO PUEDE ver\n"+
					"  un secreto de %s caracteres o menos, y necesita ≥%s. El techo de la entropía de\n"+
					"  Shannon a largo n es log2(n).\n"+
					"  Esos números tienen que estar escritos ACÁ, donde alguien calibra. Si moviste el\n"+
					"  umbral, actualizá el comentario: el piso se movió con él. Si no, el próximo que\n"+
					"  acuñe una credencial corta va a suponer que este catch-all la cubre. Ya pasó.",
					aguja, entropyThreshold, piso, ultimoInvisible, primeroVisible)
			}
		}
	})
}
