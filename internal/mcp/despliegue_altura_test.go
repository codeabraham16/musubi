package mcp

// despliegue_altura_test.go custodia el scrape del plano de APLICACIÓN de la base de Altura.
//
// Sobre la misma máquina hay ahora DOS productores, y eso está bien porque producen datos
// DISTINTOS: Musubi mide el host (CPU, memoria, disco) y este scrape mide la aplicación (la base,
// el pooler). La regla del track —«un solo productor POR DATO»— se sostiene sólo si este scrape
// NO trae `node_*`, que es justamente lo que Musubi ya produce. Si alguien la afloja, no se rompe
// nada visible: aparecen dos series de memoria para la misma máquina y las alertas salen dobles,
// que es exactamente lo que pasó al encender el empuje OTLP.

import (
	"os"
	"strings"
	"testing"
)

func leerScrapeDeAltura(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile("../../deploy/prometheus/scrapes/altura-db.yml.ejemplo")
	if err != nil {
		t.Fatalf("falta el scrape de Altura: %v", err)
	}
	return string(b)
}

// EL ARCHIVO PUEDE ESTAR EN /etc Y PROMETHEUS IGNORARLO.
//
// Es el mismo fallo que ya costó una vez con las reglas de flota: el archivo copiado, correcto, y
// Prometheus sin cargarlo porque la configuración apuntaba a otra ruta. Se ve PEOR que no
// haberlo instalado, porque se ve puesto. El scrape de sitio se carga por glob; sin la línea en
// prometheus.yml, el `.ejemplo` no sirve para nada.
//
// Sabotaje que la hace fallar: sacar el bloque `scrape_config_files` de prometheus.yml.
func TestElScrapeDeSitioLoCargaPrometheusDeVerdad(t *testing.T) {
	b, err := os.ReadFile("../../deploy/prometheus/prometheus.yml")
	if err != nil {
		t.Fatal(err)
	}
	cfg := string(b)
	if !strings.Contains(cfg, "scrape_config_files:") {
		t.Fatal("prometheus.yml no carga scrapes de sitio: el archivo de Altura quedaría en /etc sin evaluarse")
	}
	if !strings.Contains(cfg, "/etc/prometheus/scrapes/*.yml") {
		t.Error("el glob de scrapes no apunta a /etc/prometheus/scrapes/, que es donde el runbook dice que va")
	}
	// El archivo de sitio necesita la clave `scrape_configs:` adentro: una lista pelada es YAML
	// válido y Prometheus la rechaza con «cannot unmarshal !!seq into config.ScrapeConfigs».
	// Verificado contra promtool 3.1.0 antes de escribir esto.
	if !strings.Contains(leerScrapeDeAltura(t), "scrape_configs:") {
		t.Error("el archivo de sitio no tiene la clave `scrape_configs:`: Prometheus lo rechaza al arrancar")
	}
}

// ESTE SCRAPE NO PUEDE TRAER LO QUE MUSUBI YA PRODUCE.
//
// `node_*` son los vitales del host y de ésos el productor es Musubi, que además los trae con la
// identidad de la flota. Traerlos también por acá daría dos series por métrica para la misma
// máquina, distinguidas nada más que por `job` — y cada regla de flota matchearía las dos.
//
// Sabotaje que la hace fallar: agregar `node_.*` al regex del `keep`.
func TestElScrapeDeAlturaNoTraeLosVitalesQueProduceMusubi(t *testing.T) {
	conf := leerScrapeDeAltura(t)

	// Se busca el regex del `keep` y se mira ESE, no el archivo entero: los comentarios de arriba
	// nombran `node_*` para explicar por qué no está, y un `strings.Contains` sobre todo el texto
	// se quedaría tranquilo con la explicación mientras la regla dice lo contrario.
	//
	// (La primera versión de esta prueba tenía justamente ese error, y peor: un lazo que hacía
	// `return` al encontrar `node_` en un regex — o sea que el sabotaje la hacía PASAR.)
	var keep string
	lineas := strings.Split(conf, "\n")
	for i, l := range lineas {
		if strings.TrimSpace(l) != "action: keep" {
			continue
		}
		// El regex del bloque está en las líneas de arriba, dentro del mismo ítem.
		for j := i - 1; j >= 0 && j > i-5; j-- {
			if t := strings.TrimSpace(lineas[j]); strings.HasPrefix(t, "regex:") {
				keep = t
				break
			}
		}
	}
	if keep == "" {
		t.Fatal("el scrape no tiene un `action: keep` con su regex: trae las 1382 series del endpoint, " +
			"de las que 658 son cubetas de histogramas que nadie de acá mira")
	}

	if strings.Contains(keep, "node_") {
		t.Errorf("el keep deja pasar los vitales del host: %s\n"+
			"De ésos el productor es Musubi, y traerlos también por acá da dos series por métrica "+
			"para la misma máquina — el mismo daño que la duplicación del empuje OTLP", keep)
	}

	// Y `up` TIENE que sobrevivir. Los metric_relabel_configs se aplican también a las series
	// sintéticas, así que un keep que no la nombre la borra — y sin `up`, un endpoint caído se ve
	// idéntico a uno sano: sin series, sin alerta, y `AlturaEndpointMudo` no dispara nunca.
	if !strings.Contains(keep, "up") {
		t.Errorf("el keep no conserva `up`: un endpoint caído quedaría indistinguible de uno sano: %s", keep)
	}
}

// LA CREDENCIAL VA POR REFERENCIA, NUNCA ADENTRO DEL ARCHIVO.
//
// Es el mismo patrón que el job del cerebro. `credentials:` acepta el secreto inline y funciona
// igual de bien, y por eso es la trampa: el archivo se versiona, se copia a un ticket y se pega
// en un chat.
//
// Sabotaje que la hace fallar: cambiar `credentials_file:` por `credentials:`.
func TestLaCredencialDelScrapeVaPorReferencia(t *testing.T) {
	conf := leerScrapeDeAltura(t)
	if !strings.Contains(conf, "credentials_file:") {
		t.Error("el scrape no usa credentials_file")
	}
	for _, l := range strings.Split(conf, "\n") {
		t2 := strings.TrimSpace(l)
		if strings.HasPrefix(t2, "credentials:") {
			t.Errorf("el secreto viaja inline en el archivo: %s", t2)
		}
	}
	// Y la referencia real del proyecto no viaja en el repo: el archivo es un `.ejemplo` con un
	// marcador. Si alguien commitea el suyo, esto lo caza.
	if !strings.Contains(conf, "TU-REFERENCIA") {
		t.Error("el ejemplo perdió su marcador: ¿se commiteó la referencia real del proyecto?")
	}
}

// EL DENOMINADOR DE UN UMBRAL SALE DE LOS DATOS, NO DE UN NÚMERO TIPEADO.
//
// Ésta es la lección del alerter que estas reglas reemplazan. Tenía
// `("pooler_conns", 350, "Conexiones del pooler (de 400)")`: dos números escritos a mano, y el
// numerador era del lado SERVIDOR mientras el 400 era el límite del lado CLIENTE. Dos pools
// distintos. La alerta vigilaba algo que tendría que multiplicarse por siete para tocar su
// umbral — nunca sonó, y «nunca sonó» se lee igual que «todo bien».
//
// Sabotaje que la hace fallar: reemplazar la métrica del denominador por un 400 literal.
func TestElUmbralDelPoolerNoTieneElTechoTipeado(t *testing.T) {
	b, err := os.ReadFile("../../deploy/musubi-alerts-altura.yml")
	if err != nil {
		t.Fatal(err)
	}
	// SE MIRAN LAS REGLAS, NO EL ARCHIVO. El encabezado de ese YAML EXPLICA el error del alerter
	// viejo y nombra `pgbouncer_config_max_client_connections` en un comentario — así que un
	// Contains sobre el texto entero se queda tranquilo con la explicación mientras la regla dice
	// otra cosa. La primera versión de esta prueba tenía ese error y el sabotaje la dejó en verde.
	var utiles []string
	for _, l := range strings.Split(string(b), "\n") {
		if t := strings.TrimSpace(l); t != "" && !strings.HasPrefix(t, "#") {
			utiles = append(utiles, t)
		}
	}
	reglas := strings.Join(utiles, "\n")
	if !strings.Contains(reglas, "pgbouncer_config_max_client_connections") {
		t.Error("el umbral del pooler no divide por el límite que publica el propio pooler: " +
			"si Supabase cambia el plan, la alerta caduca sin avisar")
	}
	// Y el numerador es el del lado CLIENTE, que es lo que ese límite acota de verdad.
	if !strings.Contains(reglas, "pgbouncer_used_clients") {
		t.Error("el numerador no son los clientes conectados: es el error exacto del alerter viejo")
	}
	// La regla que sostiene a las otras tres: sin ella, un endpoint caído las apaga en silencio.
	if !strings.Contains(reglas, `up{job="altura-db"} == 0`) {
		t.Error("falta la alerta del endpoint mudo: sin ella las otras tres enmudecen sin fallar")
	}
}
