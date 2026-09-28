package config

import (
	"os"
	"strings"
	"testing"
)

// TestLaMemoriaDeOtrosProyectosLlegaDesdeElYAML: las dos claves de loop que deciden cuánto de otros
// proyectos traen los hooks llegan al campo por el parser; una sección `loop:` escrita antes de que
// existieran —todas las que hay— cae al default del dueño («aparte» con 2) y no al vacío; sin la
// sección, también; y escritas vacías, o con el tope en 0, también. Y las dos claves están
// documentadas donde se las va a buscar.
//
// Sabotaje: la etiqueta yaml del modo cambia de nombre y la clave del archivo cae al default en silencio.
// arnes: archivo="internal/config/config.go"
// arnes: de="RecallOtrosProyectos string `yaml:\"recall_otros_proyectos\"`"
// arnes: a="RecallOtrosProyectos string `yaml:\"otros_saboteado\"`"
//
// Sabotaje: el modo escrito vacío deja el campo vacío. Una sección SIN la clave no llega a esa línea
// —Parse arranca de Default() y yaml sólo pisa lo escrito—, así que el caso que la muerde es el de la
// clave escrita en "".
// arnes: archivo="internal/config/config.go"
// arnes: de="\t\t\tc.Loop.RecallOtrosProyectos = d.Loop.RecallOtrosProyectos\n"
// arnes: a="\t\t\tc.Loop.RecallOtrosProyectos = \"\"\n"
//
// Sabotaje: la clave del tope desaparece de config.example.yaml.
// arnes: archivo=".musubi/config.example.yaml"
// arnes: de="#   recall_otros_max: 2"
// arnes: a="#   recall_otros_saboteado: 2"
func TestLaMemoriaDeOtrosProyectosLlegaDesdeElYAML(t *testing.T) {
	for _, c := range []struct {
		nombre, yaml string
		modo         string
		tope         int
	}{
		{"las dos claves", "loop:\n  recall_otros_proyectos: aislado\n  recall_otros_max: 5\n", OtrosProyectosAislado, 5},
		{"un negativo llega tal cual", "loop:\n  recall_otros_max: -1\n", OtrosProyectosAparte, -1},
		{"la sección sin las claves ⇒ default", "loop:\n  per_turn_recall: true\n", OtrosProyectosAparte, 2},
		{"escritas vacías ⇒ default", "loop:\n  recall_otros_proyectos: \"\"\n  recall_otros_max: 0\n", OtrosProyectosAparte, 2},
		{"sin sección ⇒ default", "memory:\n  team_mode: true\n", OtrosProyectosAparte, 2},
	} {
		cfg, err := Parse([]byte(c.yaml))
		if err != nil {
			t.Fatalf("%s: el YAML no parsea: %v", c.nombre, err)
		}
		if cfg.Loop.RecallOtrosProyectos != c.modo || cfg.Loop.RecallOtrosMax != c.tope {
			t.Errorf("%s: se escribió\n%s\ny quedó modo=%q tope=%d, esperaba %q y %d",
				c.nombre, c.yaml, cfg.Loop.RecallOtrosProyectos, cfg.Loop.RecallOtrosMax, c.modo, c.tope)
		}
	}

	b, err := os.ReadFile("../../.musubi/config.example.yaml")
	if err != nil {
		t.Fatal(err)
	}
	for _, clave := range []string{"recall_otros_proyectos:", "recall_otros_max:"} {
		if !strings.Contains(string(b), clave) {
			t.Errorf("config.example.yaml no documenta %q: existe en el binario y no donde alguien la va a buscar", clave)
		}
	}
}
