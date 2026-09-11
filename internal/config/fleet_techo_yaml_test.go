package config

// fleet_techo_yaml_test.go custodia LA PUERTA de la perilla del techo de servicios del export:
// el nombre con el que se escribe en `.musubi/config.yaml`.
//
// ════════════════════════════════════════════════════════════════════════════════════════════
// POR QUÉ HACÍA FALTA UNA PRUEBA QUE PARSEE YAML DE VERDAD
//
// Las dos pruebas que había (`TestElTechoDeServiciosDistingueElDefaultDelApagado` y la del
// exportador) construyen el `FleetConfig` con un LITERAL DE GO: `FleetConfig{ServicesPerProject
// Export: 300}`. Ninguna de las dos pasa por el parser. Medido: se le cambió el nombre a la
// etiqueta `yaml:` del campo y las dos siguieron en verde — o sea que la perilla dejaba de
// existir desde el archivo (su ÚNICA puerta real) y caía en silencio al default, con el operador
// leyendo su propio `services_per_project_export: 300` en el YAML y el binario ignorándolo.
//
// Un campo de configuración tiene DOS contratos y las pruebas de literal sólo tocan uno: qué hace
// el número, y CÓMO SE ESCRIBE. El segundo es texto publicado —está en config.example.yaml y en
// el RUNBOOK— y por eso se prueba contra ese texto y no contra la etiqueta, que es justo lo que
// el sabotaje mueve.

import (
	"os"
	"reflect"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// claveDelTechoDeServicios es el nombre PUBLICADO de la perilla. No sale de la etiqueta `yaml:`
// a propósito: derivarlo de ahí haría que renombrar la etiqueta renombrara también la prueba, y
// el sabotaje pasaría de largo. Sale de la documentación, que es lo que el operador tipea.
const claveDelTechoDeServicios = "services_per_project_export"

// El YAML que un operador escribe tiene que llegar al campo. Es la prueba que faltaba: entra por
// el parser, no por un literal de Go.
//
// Sabotaje que la pone roja: cambiar la etiqueta `yaml:` de ServicesPerProjectExport en
// config.go (con las dos pruebas de literal quedaba todo verde).
func TestElTechoDeServiciosLlegaDesdeElYAMLYNoSoloDesdeUnLiteralDeGo(t *testing.T) {
	casos := []struct {
		nombre  string
		yaml    string
		esperar int // el EFECTIVO, que es el que gobierna el export
	}{
		{
			nombre:  "un número",
			yaml:    "fleet:\n  " + claveDelTechoDeServicios + ": 300\n",
			esperar: 300,
		},
		{
			nombre:  "negativo ⇒ sin techo",
			yaml:    "fleet:\n  " + claveDelTechoDeServicios + ": -1\n",
			esperar: 0,
		},
		{
			// La sección existe y la clave no: tiene que dar el DEFAULT, no «sin techo». Es el
			// caso que separa «no lo escribí» de «lo apagué», y los dos rompen para lados
			// opuestos.
			nombre:  "la sección sin la clave ⇒ default",
			yaml:    "fleet:\n  probe_minutes: 2\n",
			esperar: Default().Fleet.EffectiveServicesPerProjectExport(),
		},
	}
	for _, c := range casos {
		var cfg Config
		if err := yaml.Unmarshal([]byte(c.yaml), &cfg); err != nil {
			t.Fatalf("%s: el YAML no parsea: %v", c.nombre, err)
		}
		if got := cfg.Fleet.EffectiveServicesPerProjectExport(); got != c.esperar {
			t.Errorf("%s: se escribió\n%s\ny el techo efectivo quedó en %d, esperaba %d. La perilla no entra por el archivo: cae al default en silencio y quien la escribió no tiene forma de enterarse.",
				c.nombre, c.yaml, got, c.esperar)
		}
	}
}

// LA ETIQUETA Y LA DOCUMENTACIÓN NO PUEDEN DIVERGIR, y acá el ancla es al revés que arriba: se
// DERIVA la etiqueta del struct por reflexión y se exige que sea la que está documentada. Las dos
// mitades se cierran entre sí — renombrar la etiqueta rompe ésta, y renombrar la constante de
// arriba (para «arreglar» ésta) rompe la de arriba, porque los archivos siguen diciendo el nombre
// viejo.
func TestElNombreDeLaPerillaEsElQueDiceLaDocumentacion(t *testing.T) {
	campo, ok := reflect.TypeOf(FleetConfig{}).FieldByName("ServicesPerProjectExport")
	if !ok {
		t.Fatal("FleetConfig ya no tiene el campo ServicesPerProjectExport")
	}
	etiqueta := strings.Split(campo.Tag.Get("yaml"), ",")[0]
	if etiqueta != claveDelTechoDeServicios {
		t.Errorf("la etiqueta yaml del campo es %q y la documentación (y el operador) dicen %q: desde el archivo la perilla ya no existe y cae al default sin avisar",
			etiqueta, claveDelTechoDeServicios)
	}

	// Y el nombre tiene que seguir estando donde se lo va a buscar. Un rename que además
	// actualizara la constante de esta prueba se choca acá.
	for _, archivo := range []string{"../../.musubi/config.example.yaml", "../../deploy/RUNBOOK.md"} {
		b, err := os.ReadFile(archivo)
		if err != nil {
			t.Fatalf("no se pudo leer %s: %v", archivo, err)
		}
		if !strings.Contains(string(b), etiqueta) {
			t.Errorf("%s no nombra a %q: la perilla existe en el binario y no en el único lugar donde alguien la va a buscar", archivo, etiqueta)
		}
	}
}
