package fleet

// Pruebas del CRUCE (fase 5 · S14) del lado del dominio: qué se busca y cómo se rotula.

import (
	"strings"
	"testing"
)

// EL NOMBRE DE LA MÁQUINA VA PRIMERO Y SIEMPRE. Es el término más preciso que existe —lo eligió
// una persona, es único en el proyecto, no depende de que nadie haya declarado servicios—, así
// que si el tope recorta, tiene que recortar lo otro.
//
// Sabotaje: agregar los servicios antes que la máquina → con más de TerminosMax servicios, el
// nombre de la máquina queda afuera y el contexto pierde su único enlace garantizado.
func TestElNombreDeLaMaquinaSiempreEsUnTermino(t *testing.T) {
	muchos := make([]string, 0, TerminosMax+10)
	for i := 0; i < TerminosMax+10; i++ {
		muchos = append(muchos, "servicio-numero-"+string(rune('a'+i)))
	}
	ts := TerminosDeContexto("altura-db", nil, muchos)

	if len(ts) == 0 {
		t.Fatal("no salió ningún término")
	}
	if ts[0].Texto != "altura-db" || ts[0].De != TerminoDeMaquina {
		t.Errorf("el primer término es %+v, esperaba la máquina", ts[0])
	}
	if len(ts) > TerminosMax {
		t.Errorf("salieron %d términos, el tope es %d: cada uno es una consulta FTS", len(ts), TerminosMax)
	}
}

// EL SUFIJO DE UNIDAD SE SACA: nadie escribe `nginx.service` en una nota, escribe «nginx». Buscar
// el nombre completo no encuentra nada y el vacío se lee como «no hay nada escrito sobre nginx».
//
// Sabotaje: no sacar el sufijo → falla acá.
func TestElSufijoDeUnidadNoLlegaALaBusqueda(t *testing.T) {
	ts := TerminosDeContexto("pc", nil, []string{"nginx.service", "backup.timer", "api.altura"})
	textos := map[string]bool{}
	for _, x := range ts {
		textos[x.Texto] = true
	}
	if !textos["nginx"] || !textos["backup"] {
		t.Errorf("no se sacó el sufijo de unidad: %v", ts)
	}
	if textos["nginx.service"] || textos["backup.timer"] {
		t.Errorf("el nombre con sufijo llegó igual a la búsqueda: %v", ts)
	}
	// UN PUNTO QUE NO ES SUFIJO DE UNIDAD NO SE TOCA: en `api.altura` el punto es parte del
	// nombre, y recortarlo buscaría otra cosa. La limpieza es de cuatro sufijos conocidos, no
	// genérica.
	if !textos["api.altura"] {
		t.Errorf("se recortó un punto que no era sufijo de unidad: %v", ts)
	}
}

// No se repite un término que ya está, sin importar mayúsculas: cada término es una consulta, y
// dos consultas idénticas devuelven cada acierto dos veces con dos enlaces que dicen lo mismo.
//
// Sabotaje: deduplicar sensible a mayúsculas → falla acá.
func TestLosTerminosNoSeRepiten(t *testing.T) {
	ts := TerminosDeContexto("nginx", nil, []string{"nginx", "NGINX.service", "Nginx"})
	if len(ts) != 1 {
		t.Errorf("salieron %d términos para el mismo nombre en cuatro formas: %v", len(ts), ts)
	}
}

// Lo demasiado corto no entra: un término de dos letras hace match con media memoria y el enlace
// deja de significar nada.
func TestUnTerminoDemasiadoCortoNoEntra(t *testing.T) {
	ts := TerminosDeContexto("ab", nil, []string{"x", "ok"})
	if len(ts) != 0 {
		t.Errorf("entraron términos por debajo del mínimo (%d): %v", TerminoMinLargo, ts)
	}
	// EL MÍNIMO SE CUENTA EN RUNAS, NO EN BYTES, y el caso tiene que elegirse con cuidado: «año»
	// son 4 bytes y 3 runas, así que pasa contando de las dos formas y no prueba nada — lo
	// descubrí ejecutando el sabotaje, que quedaba verde. «ño» son 3 bytes y 2 runas: contando
	// bytes entra (3 ≥ 3) y contando runas no (2 < 3), que es la única forma de distinguirlas.
	if ts := TerminosDeContexto("ño", nil, nil); len(ts) != 0 {
		t.Errorf("un nombre de DOS runas entró: se está contando en bytes, no en caracteres: %v", ts)
	}
	// Y el control por el otro lado: tres runas SÍ entran.
	if ts := TerminosDeContexto("año", nil, nil); len(ts) != 1 {
		t.Errorf("un nombre de tres runas quedó afuera: %v", ts)
	}
}

// Los huecos declaran las DOS cosas que hacen falta para leer bien la respuesta: que es
// correlación y no causa, y qué significa cada tipo de enlace.
//
// Sabotaje: devolver nil → falla acá.
func TestLosHuecosDelContextoDicenQueNoEsCausa(t *testing.T) {
	huecos := HuecosDelContexto()
	if len(huecos) == 0 {
		t.Fatal("el contexto tiene que declarar sus límites")
	}
	junto := strings.ToLower(strings.Join(huecos, " | "))
	for _, obligatorio := range []string{"correlación", "no causa", "termino", "ventana", "homónimo", "inventario"} {
		if !strings.Contains(junto, obligatorio) {
			t.Errorf("falta declarar %q: %s", obligatorio, junto)
		}
	}
}

// UN SERVICIO DECLARADO POR UNA PERSONA GANA UNA RANURA ANTES QUE UNA UNIT DEL SISTEMA.
//
// Un host enumera decenas de units y el tope se llena con las primeras que vengan. Medido en
// producción: la primera corrida contra `musubi-server` gastó las doce ranuras en `avahi-daemon`,
// `NetworkManager-wait-online` y compañía, y dejó afuera `alturito20` — el único servicio del que
// alguien escribió algo alguna vez.
//
// Sabotaje: recorrer los reportados antes que los declarados → falla acá.
func TestUnServicioDeclaradoGanaLaRanuraAntesQueUnaUnitDelSistema(t *testing.T) {
	reportados := make([]string, 0, TerminosMax+5)
	for i := 0; i < TerminosMax+5; i++ {
		reportados = append(reportados, "unit-del-sistema-"+string(rune('a'+i)))
	}
	ts := TerminosDeContexto("musubi-server", []string{"alturito20"}, reportados)

	if len(ts) < 2 {
		t.Fatalf("esperaba al menos la máquina y el declarado: %v", ts)
	}
	if ts[0].Texto != "musubi-server" {
		t.Errorf("el primer término tiene que ser la máquina, es %q", ts[0].Texto)
	}
	if ts[1].Texto != "alturito20" || ts[1].De != TerminoDeServicio {
		t.Errorf("el segundo término tiene que ser el servicio DECLARADO, es %+v", ts[1])
	}
	// CONTROL: los reportados igual entran, con lo que sobra del tope. Sin esto, ignorarlos del
	// todo pasaría las aserciones de arriba.
	hayReportado := false
	for _, x := range ts {
		if strings.HasPrefix(x.Texto, "unit-del-sistema-") {
			hayReportado = true
		}
	}
	if !hayReportado {
		t.Error("los servicios reportados por la máquina también tienen que entrar")
	}
}
