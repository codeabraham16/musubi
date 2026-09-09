package fleet_test

import (
	"crypto/sha256"
	"encoding/hex"
	"reflect"
	"sort"
	"strings"
	"testing"

	"musubi/internal/buildid"
	"musubi/internal/fleet"
)

// EL CAPVER TIENE QUE MOVERSE CUANDO SE MUEVE EL CONTRATO, Y HASTA HOY NADA LO EXIGÍA.
//
// `buildid.go` dice, tres líneas arriba de la constante: «Sube cuando cambia el CONTRATO, no
// cuando cambia la versión del producto». Esa frase es la regla, y el 2026-09-09 se incumplió
// EN EL MISMO ARCHIVO donde está escrita: `663d5a0` le agregó TRES campos a `CuerpoLatido`
// —`ServiciosOmitidos`, `TokenFuente`, `ServiciosError`— y no tocó `Capver`, que seguía en el 1
// que puso `3eba3f6` el día anterior.
//
// NO FALLÓ EL CONOCIMIENTO NI LA DOCUMENTACIÓN, FALLÓ QUE NADA CONVIERTE ESA FRASE EN UNA GUARDA.
// `EnLaBanda()` custodia que un capver esté DENTRO del rango; nadie custodiaba que el rango SUBA
// cuando el contrato cambia. Un campo nuevo entraba sin que nada preguntara por la constante.
//
// LO QUE COSTÓ, MEDIDO. `davantis-1` (0.139.1, sin `ServiciosError`) y `musubi-server` (0.139.6,
// con él) declaran los dos `capver=1` y hablan contratos distintos, así que el cerebro no puede
// distinguirlos. Y `musubi_fleet_device_services_unknown` devuelve 0 incondicional cuando
// `ServiciosError == ""`, o sea que un agente que NO CONOCE el campo se ve igual que uno que
// enumeró bien: `MaquinaNoPuedeEnumerar` queda verde por ignorancia. Como dispara con `== 1`, el
// falso 0 es una alerta PERDIDA, no una falsa — silenciosa.
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// POR QUÉ UN PIN DEL CONJUNTO DE CAMPOS Y NO UN CONTEO
//
// La forma obvia —contar campos contra líneas de bitácora— NO PASA el sabotaje inverso: subir
// `Capver` y agregar su línea SIN tocar ningún campo la pondría roja, y eso es un cambio
// legítimo (el contrato puede moverse en otro lado, como un método MCP nuevo). Una guarda así
// custodia «dos archivos cambian juntos», que es otra cosa y se satisface haciendo ruido.
//
// Fijar el CONJUNTO a un valor de capver pasa los dos: se compara contra el pin del capver más
// alto que no supere `buildid.Capver`, así que un bump sin pin nuevo CAE al anterior y queda
// verde si los campos no se movieron. La idea es de la sesión `musubi-89`.
//
// SE FIJAN LOS TAGS JSON Y LOS TIPOS, NO LOS NOMBRES DE CAMPO. Lo que viaja por la red es el tag:
// renombrar el campo Go dejando el tag igual NO es un cambio de contrato, y cambiar el tag
// dejando el nombre SÍ lo es. Con los nombres se tendría un rojo falso en el primer caso y —peor—
// un VERDE en el segundo, que es una ruptura real. El tag entra COMPLETO, con `omitempty`
// incluido: agregarlo cambia lo que se ve del otro lado cuando el valor es cero.
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// NO HAY `pin[1]`, Y NO ES UN OLVIDO
//
// A la hora en que se declaró `Capver = 1` (`3eba3f6`, 2026-09-08 09:09) `CuerpoLatido` NO EXISTÍA
// como tipo: nació once minutos después, en `77c7ca5` (#419, 09:20), cuando el sobre pasó a ser
// tipado. Medido con `git log -S"CuerpoLatido struct"`, que devuelve `77c7ca5` como primer commit
// en todo el repo. Antes de eso el cuerpo era un mapa, así que no hay un conjunto de campos que
// fijar para el capver 1.
//
// Escribir igual un `pin[1]` reconstruido sería inventar un registro histórico, y este archivo
// existe justamente porque una afirmación que nadie midió costó una alerta. La ausencia se
// declara acá en vez de rellenarse.
var pinesDelCuerpoLatido = map[int]string{
	2: "1e0f6546c2e95f8143ba0e7930fcf0297078b7634821c72a3390e71b8ba1002d",
}

// huellaDelCuerpoLatido devuelve la huella del conjunto de campos QUE VIAJAN, y la lista con la
// que se calculó — la lista va en el mensaje de error porque un sha que cambió no dice QUÉ cambió.
func huellaDelCuerpoLatido() (string, []string) {
	t := reflect.TypeOf(fleet.CuerpoLatido{})
	var campos []string
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if f.PkgPath != "" {
			continue // no exportado: no se serializa, no es contrato
		}
		tag := f.Tag.Get("json")
		if tag == "-" {
			continue // excluido a mano del JSON
		}
		if tag == "" {
			tag = f.Name // sin tag, encoding/json usa el nombre
		}
		campos = append(campos, tag+"|"+f.Type.String())
	}
	// ORDENADO: el orden de declaración no se ve del otro lado, así que mover un campo de lugar
	// no es un cambio de contrato y no tiene por qué pedir un bump.
	sort.Strings(campos)
	suma := sha256.Sum256([]byte(strings.Join(campos, "\n")))
	return hex.EncodeToString(suma[:]), campos
}

func TestElCapverSubeCuandoSeMueveElContratoDelLatido(t *testing.T) {
	huella, campos := huellaDelCuerpoLatido()

	fijado := 0
	for capver := range pinesDelCuerpoLatido {
		if capver <= buildid.Capver && capver > fijado {
			fijado = capver
		}
	}
	if fijado == 0 {
		t.Fatalf("no hay ningún pin para un capver <= %d (buildid.Capver).\n"+
			"El pin más viejo del mapa es de un capver POSTERIOR al declarado, así que esta guarda "+
			"no tiene contra qué comparar. Si acabás de BAJAR Capver, no se hace: la constante es "+
			"monótona y bajarla deja a las máquinas nuevas fuera de banda sin que nadie lo decida.\n"+
			"Huella actual: %s", buildid.Capver, huella)
	}

	if pinesDelCuerpoLatido[fijado] != huella {
		t.Errorf("EL CONTRATO DEL LATIDO CAMBIÓ Y `buildid.Capver` SIGUE EN %d.\n\n"+
			"  esperado (pin del capver %d): %s\n"+
			"  actual:                       %s\n\n"+
			"Campos que viajan hoy (tag json | tipo):\n    %s\n\n"+
			"Qué hacer, y en este orden:\n"+
			"  1. Subí `buildid.Capver` a %d y agregale su línea de bitácora, que NO se borra.\n"+
			"  2. Agregá `%d: \"%s\"` a `pinesDelCuerpoLatido`, sin tocar las entradas viejas.\n\n"+
			"POR QUÉ NO ALCANZA CON QUE COMPILE: dos agentes con builds distintos pueden declarar el "+
			"mismo capver y hablar contratos distintos, y el cerebro no tiene otra forma de "+
			"distinguirlos. Pasó el 2026-09-09 — `davantis-1` y `musubi-server` declarando los dos "+
			"capver=1 con y sin `ServiciosError`— y dejó `MaquinaNoPuedeEnumerar` verde por "+
			"ignorancia sobre dos máquinas.\n"+
			"Y OJO CON `CapverMin`: subir `Capver` NO retira soporte; retirarlo es mover `CapverMin`, "+
			"que es otra decisión y tiene su propia bitácora.",
			buildid.Capver, fijado, pinesDelCuerpoLatido[fijado], huella,
			strings.Join(campos, "\n    "), buildid.Capver+1, buildid.Capver+1, huella)
	}
}

// LA BANDA TIENE QUE SER UNA BANDA. `CapverMin > Capver` la deja vacía: ningún par podría hablar
// con este binario, y `EnLaBanda()` devolvería false para todo sin que nada lo explique.
func TestLaBandaDeCapverNoPuedeEstarVacia(t *testing.T) {
	if buildid.CapverMin > buildid.Capver {
		t.Fatalf("la banda [%d, %d] está vacía: ningún capver puede caer adentro, así que este "+
			"binario rechazaría a la flota entera", buildid.CapverMin, buildid.Capver)
	}
	if !buildid.EnLaBanda(buildid.Capver) {
		t.Errorf("el binario no acepta su PROPIO capver (%d): la banda es [%d, %d]",
			buildid.Capver, buildid.CapverMin, buildid.Capver)
	}
}
