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
	// RE-FIJADO el 2026-09-11 SIN SUBIR CAPVER, y la distinción importa: el contrato NO se movió.
	// Lo que cambió es lo que la huella MIDE — antes se quedaba en `muestra|json.RawMessage` y
	// ahora baja adentro de `fleet.Muestra`. Subir Capver por esto habría sido declarar un cambio
	// de contrato que no existió, y habría dejado fuera de banda a agentes que hablan exactamente
	// lo mismo. El pin viejo era `1e0f6546…`; se reemplaza porque medía de menos, no porque haya
	// quedado obsoleto.
	2: "65eb52424c6bdb8d2b77ff474ca71dd1b8bdd16304ea3fd0f8d7284baff52097",
}

// pinesDeLaRespuesta es LA VUELTA DEL CABLE: lo que el cerebro le contesta al agente.
//
// No existía. `buildid.go` afirma que Capver gobierna «el contrato entre una máquina y el
// cerebro», y sólo estaba fijada la IDA. Un campo nuevo en `RespuestaLatido` produce el mismo
// incidente que produjo la ida: dos binarios declarando el mismo capver y entendiendo cosas
// distintas, sin que el cerebro tenga forma de distinguirlos.
var pinesDeLaRespuesta = map[int]string{
	2: "75b412b4124f13ac0b2fc548e578308852ec8209bf209acf83cfac84d7056e76",
}

// loQueLlevaCadaRawMessage dice qué viaja ADENTRO de un campo declarado `json.RawMessage`.
//
// ESTO ES LO QUE LE FALTABA AL PIN, Y ERA EL AGUJERO ENTERO. `CuerpoLatido.Muestra` está tipado
// como `json.RawMessage` —a propósito, para poder pesarla CRUDA contra su propio techo— así que
// la huella veía `muestra|json.RawMessage` y NADA MÁS. Agregarle un campo a `fleet.Muestra` —que
// es telemetría que el cerebro parsea y de la que dependen reglas de alerta— no movía el sha y no
// pedía ningún bump de Capver. El pin fijaba el SOBRE y no lo que viaja adentro.
//
// Y es justo el campo que más se mueve: cada dimensión nueva que se mide entra por ahí.
var loQueLlevaCadaRawMessage = map[string]reflect.Type{
	"muestra": reflect.TypeOf(fleet.Muestra{}),
}

// huellaDeLaForma arma la lista de campos que VIAJAN de un tipo, bajando a los tipos anidados.
//
// BAJA RECURSIVAMENTE porque el contrato no termina en el primer nivel: un campo nuevo adentro de
// una estructura anidada se ve igual del otro lado que uno nuevo arriba, y el parser del cerebro
// lo tiene que entender igual. `profundidad` corta las estructuras recursivas: sin ese tope, un
// tipo que se referencia a sí mismo colgaría la prueba en vez de fallarla.
func huellaDeLaForma(t reflect.Type, prefijo string, profundidad int, campos *[]string) {
	if profundidad > 6 {
		*campos = append(*campos, prefijo+"|...corte por profundidad")
		return
	}
	for t.Kind() == reflect.Pointer || t.Kind() == reflect.Slice || t.Kind() == reflect.Array {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return
	}
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
		if coma := strings.Index(tag, ","); coma >= 0 {
			// Las opciones del tag (`omitempty`) NO son contrato: no cambian qué entiende el otro
			// lado, sólo si el campo aparece cuando está vacío.
			tag = tag[:coma]
		}
		nombre := prefijo + tag
		*campos = append(*campos, nombre+"|"+f.Type.String())

		if adentro, hay := loQueLlevaCadaRawMessage[tag]; hay {
			huellaDeLaForma(adentro, nombre+".", profundidad+1, campos)
			continue
		}
		huellaDeLaForma(f.Type, nombre+".", profundidad+1, campos)
	}
}

// huellaDeUnTipo devuelve la huella del conjunto de campos QUE VIAJAN, y la lista con la que se
// calculó — la lista va en el mensaje de error porque un sha que cambió no dice QUÉ cambió.
func huellaDeUnTipo(t reflect.Type) (string, []string) {
	var campos []string
	huellaDeLaForma(t, "", 0, &campos)
	// ORDENADO: el orden de declaración no se ve del otro lado, así que mover un campo de lugar
	// no es un cambio de contrato y no tiene por qué pedir un bump.
	sort.Strings(campos)
	suma := sha256.Sum256([]byte(strings.Join(campos, "\n")))
	return hex.EncodeToString(suma[:]), campos
}

func huellaDelCuerpoLatido() (string, []string) {
	return huellaDeUnTipo(reflect.TypeOf(fleet.CuerpoLatido{}))
}

// huellaDeLaRespuesta es LA OTRA DIRECCIÓN DEL CABLE, que no miraba nadie.
//
// `buildid.go` dice que Capver gobierna «el contrato entre una máquina y el cerebro», y un
// contrato tiene dos puntas: lo que el agente manda y lo que el cerebro contesta. Sólo la ida
// estaba fijada. Un campo nuevo en `RespuestaLatido` —o uno que cambia de tipo— es exactamente el
// mismo modo de falla: dos binarios declarando el mismo capver y entendiendo cosas distintas.
func huellaDeLaRespuesta() (string, []string) {
	return huellaDeUnTipo(reflect.TypeOf(fleet.RespuestaLatido{}))
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

// LA VUELTA DEL CABLE TAMBIÉN SE FIJA.
//
// Sabotaje que la hace fallar: agregarle un campo a `fleet.RespuestaLatido` sin subir Capver.
func TestElCapverSubeCuandoSeMueveLaRespuestaDelCerebro(t *testing.T) {
	huella, campos := huellaDeLaRespuesta()

	fijado := 0
	for capver := range pinesDeLaRespuesta {
		if capver <= buildid.Capver && capver > fijado {
			fijado = capver
		}
	}
	if fijado == 0 {
		t.Fatalf("no hay ningún pin de la RESPUESTA para un capver <= %d: esta guarda no tiene "+
			"contra qué comparar.\nHuella actual: %s", buildid.Capver, huella)
	}
	if pinesDeLaRespuesta[fijado] != huella {
		t.Errorf("LA RESPUESTA DEL CEREBRO CAMBIÓ Y `buildid.Capver` SIGUE EN %d.\n\n"+
			"  esperado (pin del capver %d): %s\n"+
			"  actual:                       %s\n\n"+
			"Campos que viajan hoy (tag json | tipo):\n    %s\n\n"+
			"Un contrato tiene DOS puntas. Lo que el agente manda ya estaba fijado; esto es lo que "+
			"el cerebro contesta, y produce el mismo incidente: dos binarios con el mismo capver "+
			"entendiendo cosas distintas.\n"+
			"Subí `buildid.Capver` a %d con su línea de bitácora, y agregá `%d: \"%s\"` acá.",
			buildid.Capver, fijado, pinesDeLaRespuesta[fijado], huella,
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
