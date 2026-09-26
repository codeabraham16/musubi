package provision

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"musubi/internal/bootstrap"
)

// wireResult describe qué pasó con el .mcp.json del proyecto.
type wireResult struct {
	path    string
	changed bool   // ¿el contenido quedó distinto de lo que había? (idempotencia)
	aviso   string // algo que el operador tiene que saber aunque el paso haya salido bien
}

// ServidorCerebro es el nombre de la entrada del .mcp.json que conecta con el cerebro central. Lo
// leen también los permisos que escribe `musubi agente instalar`: una regla por nombre de servidor.
const ServidorCerebro = "musubi-cerebro"

// wireMCPJSON cablea (idempotente) el .mcp.json de projectDir con DOS entradas: la LOCAL
// ("musubi", stdio al binario, forma portable ${MUSUBI_BIN}) y la del CEREBRO
// ("musubi-cerebro", stdio a `musubi cerebro`, que reenvía al brain y resuelve el bearer con
// `config.SecretoDeEnv(tokenEnv)` — o sea que honra `<VAR>_FILE`). Preserva
// cualquier otra entrada/clave existente. El secreto NUNCA toca el archivo (va por ${VAR}).
// Con dryRun no escribe: solo calcula si cambiaría.
func wireMCPJSON(projectDir, brain, tokenEnv, exePath string, dryRun bool) (wireResult, error) {
	res := wireResult{path: filepath.Join(projectDir, ".mcp.json")}

	existing, err := os.ReadFile(res.path)
	if err != nil && !os.IsNotExist(err) {
		return res, fmt.Errorf("no se pudo leer %s: %w", res.path, err)
	}

	// Entrada LOCAL (portable): el command se resuelve por MUSUBI_BIN con la ruta actual de
	// fallback; el daemon toma la raíz del proyecto de CLAUDE_PROJECT_DIR (igual que `setup`).
	local := bootstrap.MCPServerEntry{
		Command: "${MUSUBI_BIN:-" + exePath + "}",
		Args:    []string{"daemon"},
	}
	merged, err := bootstrap.MergeMCPServer(existing, "musubi", local)
	if err != nil {
		return res, err
	}

	// Entrada CEREBRO (remota http): bearer por referencia a la env var (nunca el secreto).
	// LA URL SALE DEL NORMALIZADOR y no de un literal, porque esto se ESCRIBE EN DISCO: lo que
	// quede acá es lo que va a usar esa máquina para siempre. Con el esquema clavado en
	// `http://`, cada proyecto aprovisionado se llevaba una URL en claro adentro del
	// .mcp.json, y el día que el cerebro deje de escuchar en claro ninguno de ellos conecta.
	//
	// Y SE VALIDA ANTES DE ESCRIBIR: una dirección que no sirve, escrita en disco, falla mucho
	// después y en otra máquina, con un error que no nombra la causa.
	baseDelCerebro, _, ok := direccionDelCerebro(brain)
	if !ok {
		return res, fmt.Errorf("la dirección del cerebro no es usable y no se escribe nada: %q (se espera host:port, http://host:port o https://host:port)", brain)
	}
	if elCertificadoNoVaAServirParaUnaIP(baseDelCerebro) {
		res.aviso = "ojo: " + baseDelCerebro + " es https contra una IP pelada, y el certificado del tailnet lleva sólo el NOMBRE como SAN. El host MCP que lee este archivo no declara ServerName, así que no va a poder validarlo: usá el nombre del tailnet en vez de la IP"
	}
	// SE CABLEA POR STDIO Y NO CON `type: "http"`, Y NO ES UNA PREFERENCIA DE ESTILO.
	//
	// La forma remota escribía `{"type":"http","url":...,"headers":{"Authorization":"Bearer
	// ${VAR}"}}` y apoyaba en que el cliente MCP expandiera la variable y MANDARA el header. El
	// cliente de Claude Code NO manda los `headers` del .mcp.json (bug anthropics/claude-code
	// #48514) y encima intenta OAuth por descubrimiento en vez de por un 401 (#46879), así que la
	// credencial nunca llegaba y el servidor quedaba en «Failed» con un error que no nombra la
	// causa. Medido el 2026-09-21 sobre un `.mcp.json` aprovisionado por acá: `musubi-cerebro`
	// figuraba Failed, y el mismo cerebro con la misma credencial por STDIO contestó el
	// `initialize` y 75 tools.
	//
	// `musubi cerebro` existe exactamente para esto —su comentario de cabecera lo dice desde que
	// se escribió— y este sitio seguía generando la forma que ese comando vino a reemplazar.
	//
	// Y HAY UN SEGUNDO MOTIVO, que es el que lo vuelve obligatorio: por stdio la credencial la
	// resuelve `config.SecretoDeEnv`, que lee `<VAR>_FILE` ANTES que `<VAR>`. O sea que un secreto
	// guardado en un archivo 0600 —en vez de exportado al entorno, que es como seis credenciales
	// terminaron en los transcriptos— sigue funcionando. Con `${VAR}` eso era imposible: el editor
	// sólo sabe expandir variables, así que el archivo obligaba a tener el secreto en el entorno.
	//
	// La URL va explícita y no por `$MUSUBI_CENTRAL_URL`: lo que se escribe acá es lo que esa
	// máquina va a usar, y depender de una variable de entorno para el DESTINO reintroduce por la
	// otra puerta el mismo «funciona según quién lanzó el editor».
	cerebro := bootstrap.MCPServerEntry{
		Command: "${MUSUBI_BIN:-" + exePath + "}",
		Args:    []string{"cerebro", "--url", baseDelCerebro, "--token-env", tokenEnv},
	}
	merged, err = bootstrap.MergeMCPServer(merged, ServidorCerebro, cerebro)
	if err != nil {
		return res, err
	}

	res.changed = !bytes.Equal(bytes.TrimSpace(existing), bytes.TrimSpace(merged))
	if dryRun || !res.changed {
		return res, nil
	}

	if dir := filepath.Dir(res.path); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return res, fmt.Errorf("no se pudo crear %s: %w", dir, err)
		}
	}
	if err := os.WriteFile(res.path, merged, 0o644); err != nil {
		return res, fmt.Errorf("no se pudo escribir %s: %w", res.path, err)
	}
	return res, nil
}
