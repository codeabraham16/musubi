package mcp

// El guion de redespliegue verificaba la migración contra un número TIPEADO A MANO.

import (
	"os"
	"regexp"
	"strings"
	"testing"

	"musubi/internal/memory"
)

// LA COMPROBACIÓN DE MIGRACIÓN NO PUEDE VOLVER A QUEDARSE VIEJA.
//
// ════════════════════════════════════════════════════════════════════════════════════════════
// POR QUÉ ESTO ES UNA PRUEBA Y NO UN COMENTARIO
//
// `deploy/redesplegar-cerebro.sh` decía `[[ "$ESQUEMA" -ge 37 ]]`, escrito cuando la última
// migración era la 37. Entre la 37 y la 44 esa línea siguió pasando y dejó de verificar nada:
// 44 ≥ 37 es cierto, y también lo sería con la migración cortada en la 40. Nadie lo notó porque
// **una comprobación que no puede ponerse roja se ve idéntica a una que funciona**.
//
// Un comentario que diga «acordate de actualizar esto» tiene el mismo destino que el número que
// reemplaza. La única forma de que no vuelva a pasar es que el guion DERIVE el número del binario
// —`musubi version --esquema`— y que algo se ponga rojo si alguien vuelve a tipearlo.
//
// Sabotaje: volver a poner un `-ge 44` en el guion, o sacarle el `version --esquema`.
func TestElRedespliegueNoTipeaLaVersionDeEsquema(t *testing.T) {
	crudo, err := os.ReadFile("../../deploy/redesplegar-cerebro.sh")
	if err != nil {
		t.Fatalf("no pude leer el guion de redespliegue: %v", err)
	}
	guion := string(crudo)

	if !strings.Contains(guion, "version --esquema") {
		t.Error("el guion ya no le pregunta al binario a qué esquema apunta: la verificación de la migración volvió a depender de que alguien se acuerde")
	}

	// Un número comparado contra `$ESQUEMA` es exactamente la forma que se quiere prohibir. No se
	// prohíben los dígitos en general —el guion tiene timeouts y modos de archivo— sino la
	// comparación del esquema contra una constante.
	tipeado := regexp.MustCompile(`\$ESQUEMA"?\s*(-ge|-eq|-gt|==|!=)\s*"?[0-9]+`)
	if m := tipeado.FindString(guion); m != "" {
		t.Errorf("el esquema se compara contra un número tipeado (%q): eso es lo que se quedó viejo entre la 37 y la 44 sin que nadie lo viera", m)
	}
}

// Y EL BINARIO TIENE QUE SABER DECIRLO. Si `EsquemaEsperado` dejara de seguir a las migraciones,
// el guion verificaría contra un número equivocado con toda confianza.
//
// Sabotaje: que EsquemaEsperado devuelva una constante.
func TestElBinarioDiceElEsquemaAlQueApunta(t *testing.T) {
	// La lista de migraciones es la única fuente: se compara contra la MAYOR versión declarada,
	// leída del propio archivo, para que agregar una migración sin tocar nada más rompa esto si
	// EsquemaEsperado dejara de derivarse.
	crudo, err := os.ReadFile("../memory/migrations.go")
	if err != nil {
		t.Fatalf("no pude leer migrations.go: %v", err)
	}
	versiones := regexp.MustCompile(`(?m)^\s+version:\s+(\d+),`).FindAllStringSubmatch(string(crudo), -1)
	if len(versiones) == 0 {
		t.Fatal("no encontré ninguna migración declarada: el regex de esta prueba quedó viejo")
	}
	mayor := 0
	for _, v := range versiones {
		n := 0
		for _, c := range v[1] {
			n = n*10 + int(c-'0')
		}
		if n > mayor {
			mayor = n
		}
	}
	if got := memory.EsquemaEsperado(); got != mayor {
		t.Errorf("EsquemaEsperado() = %d y la migración más alta declarada es %d: el guion de despliegue verificaría contra un número equivocado", got, mayor)
	}
}
