package mcp

// despliegue_inodos_test.go custodia UNA cosa: que `preparar.sh` no le reemplace el inodo a un
// archivo que un contenedor tiene bind-montado.
//
// ════════════════════════════════════════════════════════════════════════════════════════════
// EL DEFECTO, Y POR QUÉ NO SE VE
//
// Un bind-mount de ARCHIVO se pega al INODO, no al nombre. `install`, `sed -i` y `mv` no escriben
// el archivo: lo desenlazan y crean otro. El contenedor sigue leyendo el anterior —que ya no tiene
// nombre— así que la recarga contesta 200 sobre el archivo equivocado; y en un host con SELinux el
// inodo nuevo nace con otra etiqueta y ni lo puede leer, con una recarga que contesta 500 sobre un
// archivo de dueño y modo POSIX perfectos. Las dos cosas por la misma razón, porque las dos viven
// en el inodo. La tabla medida —con testigo de hard link, que es la única forma confiable— está en
// deploy/README.md.
//
// POR QUÉ ESTA GUARDA NO INTENTA CLASIFICAR SOLA CUÁL MOUNT ES DE ARCHIVO
//
// Porque no se puede desde el texto, y una guarda que lo adivina es peor que ninguna. Un
// `${MUSUBI_PROM_DIR}/rules` y un `${MUSUBI_PROM_DIR}/prometheus.yml` se escriben igual en el
// compose y significan cosas distintas: en el mount de DIRECTORIO un inodo nuevo adentro SÍ lo ve
// el contenedor, así que ahí `install` está bien.
//
// El reparto es: LA PERSONA declara acá abajo qué es cada mount, y la PRUEBA obliga a que esa
// declaración exista y hace cumplir su consecuencia. Un mount nuevo sin declarar pone esto rojo, y
// eso es a propósito: es la decisión que nadie puede tomar en su lugar.
//
// Y RESUELVE LA INDIRECCIÓN, que es donde una versión anterior de esta idea quedaba ciega justo en
// el sitio más peligroso: `musubi.token` no se escribe por su nombre sino por `$TOKEN_FILE`, así
// que buscar «líneas que digan musubi.token y mv» no encontraba el `mv "$TOKEN_FILE.tmp"` que le
// cambiaba el inodo al token en CADA corrida. Se resuelven las asignaciones `VAR="$DEST/algo"` y se
// FALLA si aparece una que no se pueda resolver — sin eso, la próxima indirección la deja ciega en
// silencio, que es el modo de falla de toda esta familia.
// ════════════════════════════════════════════════════════════════════════════════════════════

import (
	"regexp"
	"strings"
	"testing"
)

// claseDeMount dice, por cada cosa montada desde el directorio de Prometheus, si el bind-mount es
// de ARCHIVO (el inodo importa) o de DIRECTORIO (no importa). Sale de deploy/docker/compose.yml.
var claseDeMount = map[string]string{
	"prometheus.yml":   "archivo",
	"musubi.token":     "archivo",
	"alertmanager.yml": "archivo",
	"rules":            "directorio",
	"secretos":         "directorio",
}

// verbosQueReemplazanElInodo son los medidos en deploy/README.md. `cp` NO está: escribe dentro del
// inodo cuando el destino es escribible — pero tampoco se recomienda, porque fracasa sobre un 0400
// y con `-f` vuelve a desenlazar. Acá se prohíben sólo los que reemplazan SIEMPRE.
var verbosQueReemplazanElInodo = []string{"install", "sed -i", "mv"}

var (
	montajeProm  = regexp.MustCompile(`\$\{MUSUBI_PROM_DIR\}/([A-Za-z0-9_.-]+):`)
	asignacionA  = regexp.MustCompile(`^([A-Z_][A-Z0-9_]*)="\$DEST/([A-Za-z0-9_.-]+)"`)
	usaDestCrudo = regexp.MustCompile(`^[A-Z_][A-Z0-9_]*="\$DEST[/"]`)
)

// TODO MOUNT DEL COMPOSE ESTÁ CLASIFICADO, y un mount nuevo obliga a decidir.
//
// Sabotaje que la hace fallar: agregar un volumen `${MUSUBI_PROM_DIR}/nuevo.yml:...` al compose sin
// tocar claseDeMount.
func TestTodoMountDePrometheusEstaClasificado(t *testing.T) {
	compose := leerDeploy(t, "docker", "compose.yml")
	hallados := map[string]bool{}
	for _, m := range montajeProm.FindAllStringSubmatch(compose, -1) {
		hallados[m[1]] = true
		if claseDeMount[m[1]] == "" {
			t.Errorf("compose.yml monta %q y claseDeMount no dice si es de ARCHIVO o de DIRECTORIO.\n"+
				"  No lo puede decidir esta prueba: en un mount de archivo el inodo importa y en uno "+
				"de directorio no. Declaralo, y si es de archivo escribilo con `poner` en preparar.sh", m[1])
		}
	}
	// CONTROL DE QUE MIRÓ ALGO: si el compose cambiara de forma, el regex dejaría de matchear y
	// esta prueba pasaría en verde sin haber encontrado un solo mount.
	if len(hallados) < 5 {
		t.Fatalf("se reconocieron %d mounts de ${MUSUBI_PROM_DIR} y son al menos 5: cambió el "+
			"formato del compose y esta guarda dejó de mirar", len(hallados))
	}
	// Y al revés: una clase declarada para algo que ya nadie monta es una excusa que sobrevivió a
	// su motivo, y la próxima persona la va a leer como vigente.
	for nombre := range claseDeMount {
		if !hallados[nombre] {
			t.Errorf("claseDeMount declara %q y el compose ya no lo monta: sacalo, o la declaración "+
				"queda diciendo algo que no es", nombre)
		}
	}
}

// UN MOUNT DE ARCHIVO NO SE ESCRIBE CON NADA QUE LE CAMBIE EL INODO.
//
// Sabotaje que la hace fallar: volver a `install -m 0644 ... "$DEST/prometheus.yml"`, o al
// `mv "$TOKEN_FILE.tmp" "$TOKEN_FILE"` que estuvo hasta hoy.
func TestPrepararNoLeReemplazaElInodoAUnMountDeArchivo(t *testing.T) {
	guion := leerDeploy(t, "docker", "preparar.sh")
	lineas := strings.Split(guion, "\n")

	// 1) Se resuelven las variables que apuntan adentro de $DEST. Sin esto la guarda queda ciega
	//    justo en el sitio peor: el token se escribe por $TOKEN_FILE y nunca por su nombre.
	deVariable := map[string]string{}
	for _, l := range lineas {
		l = strings.TrimSpace(l)
		if m := asignacionA.FindStringSubmatch(l); m != nil {
			deVariable["$"+m[1]] = m[2]
			continue
		}
		// FALLA CERRADA: una asignación desde $DEST que este parseo no entiende es una indirección
		// nueva, y dejarla pasar es exactamente cómo esta guarda se apagaría sola.
		if usaDestCrudo.MatchString(l) {
			t.Fatalf("preparar.sh tiene una asignación desde $DEST que esta guarda no sabe "+
				"resolver:\n    %s\n  Enseñale a resolverla o la próxima indirección la deja ciega "+
				"en silencio", l)
		}
	}
	if len(deVariable) < 2 {
		t.Fatalf("se resolvieron %d variables de $DEST y son al menos 2 (SECRETOS y TOKEN_FILE): "+
			"cambió la forma del guion y esta guarda dejó de resolver la indirección", len(deVariable))
	}

	// 2) Para cada mount de ARCHIVO, ninguna línea puede nombrarlo junto a un verbo que reemplaza.
	for n, l := range lineas {
		limpia := strings.TrimSpace(l)
		if limpia == "" || strings.HasPrefix(limpia, "#") {
			continue // los comentarios NOMBRAN los verbos a propósito, para explicar por qué no van
		}
		for _, verbo := range verbosQueReemplazanElInodo {
			if !strings.Contains(limpia, verbo+" ") {
				continue
			}
			for nombre, clase := range claseDeMount {
				if clase != "archivo" {
					continue
				}
				nombra := strings.Contains(limpia, nombre)
				for v, apunta := range deVariable {
					if apunta == nombre && strings.Contains(limpia, v) {
						nombra = true
					}
				}
				if nombra {
					t.Errorf("preparar.sh:%d escribe el mount de ARCHIVO %q con %q, que REEMPLAZA "+
						"el inodo:\n    %s\n  El contenedor se queda leyendo el archivo anterior, y "+
						"en un host con SELinux ni lo puede leer. Usá `poner`, que escribe dentro "+
						"del inodo que ya existe.", n+1, nombre, verbo, limpia)
				}
			}
		}
	}
}

// Y LOS TRES MOUNTS DE ARCHIVO SE ESCRIBEN DE VERDAD, no es que nadie los toque.
//
// La prueba de arriba es una PROHIBICIÓN, y una prohibición se cumple sola si el guion deja de
// escribir el archivo — ahí el despliegue queda roto de otra forma y la guarda en verde. Esta exige
// la contraparte: que cada uno se escriba, y por un camino que conserve el inodo.
//
// SE ACEPTA `poner` O UNA REDIRECCIÓN DIRECTA, y no sólo `poner`. La primera versión de esta prueba
// exigía `poner` y salió roja sobre el guion correcto: el token se escribe con
// `printf '%s' "$TOK" > "$TOKEN_FILE"`, que es EXACTAMENTE el mecanismo que `poner` usa adentro
// —`cat origen > destino`— así que pasar por el helper no agregaría nada. Una guarda que exige la
// forma en vez del efecto manda a envolver código que ya estaba bien.
//
// Sabotaje que la hace fallar: borrar cualquiera de los tres `poner`, o el `printf > "$TOKEN_FILE"`.
func TestLosTresMountsDeArchivoSeEscribenSinCambiarElInodo(t *testing.T) {
	guion := leerDeploy(t, "docker", "preparar.sh")
	if !strings.Contains(guion, "\nponer() {") {
		t.Fatal("preparar.sh no define `poner`: la guarda de la prohibición pasaría por ausencia")
	}
	// Las variables que apuntan adentro de $DEST, igual que arriba: el token se escribe por
	// $TOKEN_FILE y nunca por su nombre.
	deVariable := map[string]string{}
	for _, l := range strings.Split(guion, "\n") {
		if m := asignacionA.FindStringSubmatch(strings.TrimSpace(l)); m != nil {
			deVariable["$"+m[1]] = m[2]
		}
	}
	for nombre, clase := range claseDeMount {
		if clase != "archivo" {
			continue
		}
		// Los nombres por los que ese destino puede aparecer: el literal y las variables que
		// apuntan a él.
		alias := []string{nombre}
		for v, apunta := range deVariable {
			if apunta == nombre {
				alias = append(alias, regexp.QuoteMeta(v))
			}
		}
		escrito := false
		for _, a := range alias {
			// `poner ... destino`, o una redirección `> "destino"`. Las dos escriben dentro del
			// inodo; la segunda es lo que la primera hace por adentro.
			if regexp.MustCompile(`(?m)^\s*poner .*`+regexp.QuoteMeta(a)).MatchString(guion) ||
				regexp.MustCompile(`>\s*"`+a+`"`).MatchString(guion) {
				escrito = true
			}
		}
		if !escrito {
			t.Errorf("el mount de ARCHIVO %q no se escribe por ningún camino que conserve el inodo "+
				"(ni `poner`, ni una redirección): o el despliegue dejó de instalarlo, o volvió a "+
				"usar algo que lo desenlaza", nombre)
		}
	}
}
