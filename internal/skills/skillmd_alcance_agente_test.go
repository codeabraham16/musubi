package skills

import "testing"

// CADA VALOR DEL VOCABULARIO TIENE SU FRASE.
//
// El vocabulario es CERRADO (vocabularioAlcance), así que la tabla de frases puede ser completa y
// esto lo afirma: el mapa de skills que Musubi le manda al agente sale de estas frases, y un valor
// sin frase dejaría a sus skills fuera del mapa sin que nada avise.
//
// Sabotaje que la hace fallar: sacarle la frase a un valor.
// arnes: archivo="internal/skills/skillmd.go"
// arnes: de="\tTareaOrquestar:  \"la tarea sea grande y paralelizable\",\n"
// arnes: a=""
func TestCadaAlcanceTieneSuFrase(t *testing.T) {
	for _, v := range VocabularioDeAlcance() {
		if FraseDeAlcance(v) == "" {
			t.Errorf("el alcance %q no tiene frase: sus skills no pueden aparecer en el mapa del agente", v)
		}
	}
	if FraseDeAlcance("phase:inventada") != "" {
		t.Error("un valor fuera del vocabulario devolvió una frase")
	}
}
