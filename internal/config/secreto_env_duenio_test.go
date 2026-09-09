package config

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// LA REGLA `<VAR>_FILE` TIENE DUEÑO: NADIE LEE UNA CREDENCIAL CON `os.Getenv` PELADO.
//
// EL DEFECTO, MEDIDO EL 2026-09-08.
//
// La config nombra credenciales con TRES campos: `auth_token_env`, `api_key_env` y
// `marketplace_api_key_env`. El primero pasaba por `SecretoDeEnv` en DIEZ sitios. Los otros dos,
// en CERO — más `ANTHROPIC_API_KEY` en `calibrate.go`, que no sale de la config y tenía el mismo
// agujero. Cuatro caminos leyendo credenciales sin la regla que los otros diez sí tienen.
//
// A89 documenta el daño: `deploy/musubi-tool.sh` recomienda `<VAR>_FILE` como LO RECOMENDADO, y
// seguir el consejo en un camino que no lo lee deja al proceso sin credencial. En
// `embedding/factory.go` era peor todavía que el A89 original: el provider se construía SIN error
// con la key vacía, así que el fallo llegaba después, lejos, disfrazado de «el backend rechaza».
//
// ESTA ES LA GUARDA QUE CONVIERTE LA REGLA EN UNA REGLA. Sin ella, la corrección de hoy es una
// lista de cuatro arreglos que el próximo camino nuevo no hereda — que es exactamente la forma de
// «la lección aprendida de un lado y no del hermano», el defecto dominante de este repo.
//
// LO QUE NO CUBRE, DECLARADO: dos sitios leen `os.Getenv` sobre nombres de credencial y NO son
// este defecto. Están en la lista de abajo con su razón, una por una, porque una excepción por
// regla («si el archivo se llama así, no mires») le daría vía libre al próximo.
//
// Sabotajes que la ponen en rojo (verificados):
//   - volver `embedding/factory.go` a `os.Getenv(envName)`;
//   - volver `methods.go` (marketplace) o `catalog.go` a `os.Getenv(apiKeyEnv)`;
//   - volver `calibrate.go` a `os.Getenv("ANTHROPIC_API_KEY")`;
//   - agregar un `os.Getenv("LO_QUE_SEA_API_KEY")` nuevo en cualquier parte de internal/ o cmd/.
func TestNadieLeeUnaCredencialConGetenvPelado(t *testing.T) {
	// EXCEPCIONES, UNA POR UNA, CON SU RAZÓN MEDIDA.
	exentos := map[string]string{
		// Es la implementación de la regla: acá el `os.Getenv` es el punto del contrato.
		"internal/config/secreto_env.go": "es SecretoDeEnv en persona",

		// NO es el mismo defecto. `cargarCredencial` lee un archivo MULTI-TOKEN para poder
		// rotar (`tokensDeArchivo` devuelve varios), y `SecretoDeEnv` RECHAZA a propósito un
		// archivo de varias líneas — son dos contratos distintos sobre el mismo sufijo. Usar
		// SecretoDeEnv acá rompería la rotación del agente, que es lo que el archivo existe para
		// permitir. Ya implementa la precedencia archivo-antes-que-variable, y lo documenta.
		"cmd/musubi/agent_token.go": "archivo multi-token para rotación; SecretoDeEnv rechaza multilínea por contrato",

		// NO es el mismo defecto: no lee un secreto, deriva un DIRECTORIO a partir de la ruta
		// del archivo de token (`filepath.Dir(tok)`) para decidir dónde dejar la marca de
		// respaldo de pantalla. El valor nunca se usa como credencial.
		"cmd/musubi/pantalla_respaldo.go": "usa la ruta para derivar un directorio, no lee el secreto",
	}

	// DOS PATRONES, Y EL SEGUNDO ES EL QUE HACE QUE ESTO SIRVA.
	//
	// La primera versión de esta guarda miraba SÓLO el argumento de `os.Getenv`, y su propio
	// sabotaje la dejó en VERDE: `apiKey := os.Getenv(envName)` —que es exactamente el defecto
	// original de `embedding/factory.go`— no tiene «KEY» adentro del paréntesis, porque el nombre
	// viaja en una variable. La guarda escrita para cazar el defecto no cazaba el defecto.
	//
	// Es el mismo error que denuncia, un piso más arriba: cubría el camino donde el nombre es un
	// literal y no el camino donde es una variable — N-1 de N. Sólo el sabotaje lo mostró.
	//
	// Por eso se mira también A QUIÉN SE LE ASIGNA: si el resultado de un `os.Getenv` termina en
	// algo que se llama `apiKey`, `token` o `secret`, es una credencial, se llame como se llame la
	// variable de entorno.
	porElNombreDeLaVariableDeEntorno := regexp.MustCompile(`(?i)os\.Getenv\(\s*"?[a-z0-9_."]*(key|token|secret|password|passwd|credential)`)
	porElDestinoDeLaAsignacion := regexp.MustCompile(`(?i)\b[a-z0-9_]*(apikey|api_key|token|secret|password|passwd|credential|clave)[a-z0-9_]*\s*:?=[^=]*os\.Getenv\(`)

	// SE BARREN LAS RAICES DE CODIGO Y NO EL REPO ENTERO. Barrer desde la raiz levantaba .go de
	// `.musubi/backups/` —copias de rescate de worktrees, que no son codigo de esta rama— y de
	// cualquier otro artefacto que caiga en el arbol. Un ofensor falso gasta el mismo tiempo que
	// uno real y ademas ensena a ignorar la guarda.
	raices := []string{filepath.Join("..", "..", "internal"), filepath.Join("..", "..", "cmd")}
	var ofensores []string
	archivosMirados := 0

	for _, raiz := range raices {
		err := filepath.Walk(raiz, func(ruta string, info os.FileInfo, err error) error {
			if err != nil {
				return nil // un directorio ilegible no puede apagar la guarda: sigue
			}
			if info.IsDir() {
				// El árbol de worktrees de agentes es una copia del repo: mirarlo multiplicaría
				// cada hallazgo por 33 y haría fallar por archivos que no son de esta rama.
				base := info.Name()
				if base == ".git" || base == ".claude" || base == "node_modules" || base == "specs" {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(ruta, ".go") || strings.HasSuffix(ruta, "_test.go") {
				return nil
			}
			rel := filepath.ToSlash(strings.TrimPrefix(filepath.ToSlash(ruta), "../../"))
			if _, ok := exentos[rel]; ok {
				return nil
			}
			crudo, err := os.ReadFile(ruta)
			if err != nil {
				return nil
			}
			archivosMirados++
			for i, linea := range strings.Split(string(crudo), "\n") {
				recortada := strings.TrimSpace(linea)
				// Los comentarios se descartan ANTES de mirar: sin esto, un comentario que
				// EXPLIQUE el defecto («acá había un os.Getenv(API_KEY) pelado») pondría la
				// guarda en rojo sobre un archivo correcto — que es el otro lado del error de
				// A107, y cuesta igual de caro.
				if strings.HasPrefix(recortada, "//") {
					continue
				}
				if porElNombreDeLaVariableDeEntorno.MatchString(linea) || porElDestinoDeLaAsignacion.MatchString(linea) {
					ofensores = append(ofensores, rel+":"+itoaLocal(i+1)+"  "+recortada)
				}
			}
			return nil
		})
		if err != nil {
			t.Fatalf("no se pudo barrer %s: %v", raiz, err)
		}
	}

	// SIN ESTO LA GUARDA SE APAGA SOLA: si el barrido deja de encontrar archivos —ruta movida,
	// SkipDir de más— `ofensores` queda vacío y esto pasa en VERDE sin haber mirado nada.
	if archivosMirados < 100 {
		t.Fatalf("sólo se miraron %d archivos .go; se esperaban más de 100. El barrido dejó de "+
			"mirar y un verde acá no significaría nada", archivosMirados)
	}

	if len(ofensores) > 0 {
		t.Errorf("hay %d lectura(s) de credencial con `os.Getenv` pelado, sin pasar por "+
			"`config.SecretoDeEnv` y sin declararse en `exentos`:\n  %s\n\n"+
			"`SecretoDeEnv` honra `<VAR>_FILE`, que `deploy/musubi-tool.sh:21` recomienda como "+
			"LO RECOMENDADO. Un camino que no lo lee deja sin credencial a quien siga el consejo "+
			"(A89), y si además construye el cliente sin error, el fallo aparece lejos y "+
			"disfrazado de otra cosa (A101).\n"+
			"Si esta lectura NO es una credencial, o tiene un contrato distinto, agregala a "+
			"`exentos` con su razón.",
			len(ofensores), strings.Join(ofensores, "\n  "))
	}
}

func itoaLocal(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}
