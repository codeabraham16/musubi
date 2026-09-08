package mcp

import (
	"context"
	"database/sql"
	"encoding/json"
	"path/filepath"
	"testing"

	"musubi/internal/buildid"
	"musubi/internal/config"
	"musubi/internal/embedding"
	"musubi/internal/memory"
	"musubi/internal/memory/memtest"

	_ "modernc.org/sqlite"
)

// I1 — EL ESQUEMA QUE PUBLICA LA IDENTIDAD ES AQUEL AL QUE ESTE BINARIO MIGRA UNA BASE DE VERDAD.
//
// POR QUÉ SE COMPARA CONTRA LA BASE Y NO CONTRA memory.EsquemaEsperado(). Comparar el campo
// contra la función que lo alimenta sería medir el proxy: las dos puntas serían la misma
// expresión escrita dos veces, y un entero tipeado a mano que HOY coincida pasaría sin hacer
// ruido. Este repo ya pagó tres veces ese error (ver la nota de tests-que-esperan-el-proxy).
//
// Acá se abre una base REAL, se la deja migrar y se lee su PRAGMA user_version, que es el
// contrato observable: si alguien reemplaza la derivación por una constante, el test se pone
// rojo el día que se agregue una migración — que es exactamente el día que importa, porque es
// cuando el número publicado empezaría a mentirle a la otra punta.
//
// EL LÍMITE, DECLARADO. Este test NO puede distinguir «derivado» de «tipeado con el valor que
// hoy es correcto»: las dos cosas dan el mismo número. Se descubrió sabotéandolo — el primer
// sabotaje escribió a mano el esquema actual y el test quedó verde, vacuo. Ninguna prueba en
// un solo instante puede separar esas dos hipótesis. Lo que este test garantiza es DETECCIÓN
// EN LA DERIVA, y eso alcanza: una constante tipeada sólo hace daño cuando el valor real se
// mueve, y ése es justo el momento en que se pone roja.
func TestI1ElEsquemaPublicadoEsElQueMigraLaBase(t *testing.T) {
	dir := t.TempDir()
	engine, err := memory.NewDbEngine(dir)
	if err != nil {
		t.Fatalf("NewDbEngine: %v", err)
	}
	t.Cleanup(func() { engine.Close() })

	db, err := sql.Open("sqlite", filepath.Join(dir, config.DirName, config.DBFile))
	if err != nil {
		t.Fatalf("abrir la base ya migrada: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	var enDisco int
	if err := db.QueryRow(`PRAGMA user_version`).Scan(&enDisco); err != nil {
		t.Fatalf("leer user_version: %v", err)
	}
	// Un cero acá no es «coincidieron»: es que la base no migró, y entonces el test no probaría
	// nada. Se distingue a propósito de un fallo de comparación.
	if enDisco == 0 {
		t.Fatal("la base quedó en user_version=0: no migró, así que este test no probaría nada")
	}

	s := NewMcpServer(engine, dir, embedding.NoopProvider{})
	if got := s.Identity().Schema; got != enDisco {
		t.Errorf("Identity().Schema = %d, pero la base migrada quedó en user_version = %d", got, enDisco)
	}
}

// I2 — EL HANDSHAKE PUBLICA LA IDENTIDAD DE ESTE BINARIO, NO UN LITERAL.
//
// La versión de prueba es deliberadamente absurda: si el handshake volviera a tipear cualquier
// cosa —"1.0.0" u otra—, no hay forma de que coincida por casualidad.
func TestI2ElHandshakePublicaLaVersionReal(t *testing.T) {
	const laVersion = "9.9.9-ningun-humano-tipearia-esto"

	s := NewMcpServer(memtest.NuevoEngine(t, t.TempDir()), t.TempDir(), embedding.NoopProvider{}, WithVersion(laVersion))
	resp, ok := s.Dispatch(context.Background(), JsonRpcRequest{JsonRpc: "2.0", ID: 1, Method: "initialize"})
	if !ok {
		t.Fatal("initialize no produjo respuesta")
	}
	crudo, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("serializar la respuesta: %v", err)
	}

	var sobre struct {
		Result struct {
			ServerInfo map[string]string `json:"serverInfo"`
			Meta       struct {
				Identity buildid.Identity `json:"musubi/identity"`
			} `json:"_meta"`
		} `json:"result"`
	}
	if err := json.Unmarshal(crudo, &sobre); err != nil {
		t.Fatalf("parsear la respuesta: %v", err)
	}

	if got := sobre.Result.ServerInfo["version"]; got != laVersion {
		t.Errorf("serverInfo.version = %q, esperaba la versión inyectada %q", got, laVersion)
	}
	id := sobre.Result.Meta.Identity
	if id.Version != laVersion {
		t.Errorf("_meta identity.version = %q, esperaba %q", id.Version, laVersion)
	}
	// Acá SÍ vale comparar contra la función: I1 ya ató la función al user_version de una base
	// real, así que lo que se prueba en este test es el TRANSPORTE, no la derivación.
	if id.Schema != memory.EsquemaEsperado() {
		t.Errorf("_meta identity.schema = %d, esperaba %d", id.Schema, memory.EsquemaEsperado())
	}
	if id.Capver != buildid.Capver {
		t.Errorf("_meta identity.capver = %d, esperaba %d", id.Capver, buildid.Capver)
	}
	if id.ToolsCount == 0 || id.CatalogSHA == "" {
		t.Errorf("la identidad viajó sin catálogo: tools=%d sha=%q", id.ToolsCount, id.CatalogSHA)
	}
}

// I3 — EL CATÁLOGO QUE DECLARA LA IDENTIDAD ES EL QUE SIRVE tools/list.
//
// Si estos dos números pudieran separarse, la identidad describiría un catálogo que nadie sirve
// y la comparación entre dos máquinas daría una deriva inventada.
func TestI3ElCatalogoDeLaIdentidadEsElQueSirveToolsList(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})

	lista, ok := s.handleToolsList().(map[string]interface{})["tools"].([]Tool)
	if !ok {
		t.Fatal("handleToolsList no devolvió []Tool: cambió la forma de la respuesta")
	}
	if got := s.Identity().ToolsCount; got != len(lista) {
		t.Errorf("Identity().ToolsCount = %d, tools/list sirve %d", got, len(lista))
	}
}

// I4 — LA HUELLA DEL CATÁLOGO DEPENDE DEL CONTENIDO.
//
// Sin este test, un sha constante —o uno calculado sólo sobre los nombres— pasaría I2 y I3 sin
// despeinarse, y la huella no serviría para lo único que se le pide: detectar que dos binarios
// describen la misma tool de dos maneras distintas, que es el defecto medido en la malla.
func TestI4LaHuellaDependeDelContenidoYNoSoloDelNombre(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	_, original := huellaDelCatalogo(s.tools)

	copia := make([]toolEntry, len(s.tools))
	copy(copia, s.tools)
	tocada := false
	for i := range copia {
		if copia[i].dormant && !toolsAllEnabled() {
			continue // no entra a la huella, tocarla no probaría nada
		}
		copia[i].Description += " (una coma de más)"
		tocada = true
		break
	}
	if !tocada {
		t.Fatal("no hay ninguna tool visible: el catálogo está vacío y el test no probaría nada")
	}

	if _, movida := huellaDelCatalogo(copia); movida == original {
		t.Errorf("cambiar una descripción no movió la huella (%s): la huella no mira el contenido", original)
	}
}

// I5 — LA CLI Y EL SERVIDOR MIDEN EL MISMO CATÁLOGO.
//
// Son dos caminos —uno sobre el registro ya construido del servidor vivo, otro armando uno al
// vuelo— y tienen que dar lo mismo. Este repo ya perdió dos features en silencio por tener dos
// cálculos separados de una misma struct de cable (internal/fleet/protocolo.go). De paso, este
// test es el único lugar donde se ejerce CatalogFingerprint sobre un McpServer sin inicializar:
// si alguna entrada del registro empieza a leer estado del servidor, acá revienta.
func TestI5LaCLIYElServidorMidenElMismoCatalogo(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	nServidor, shaServidor := huellaDelCatalogo(s.tools)
	nCLI, shaCLI := CatalogFingerprint()

	if nCLI != nServidor {
		t.Errorf("CatalogFingerprint cuenta %d tools y el servidor %d", nCLI, nServidor)
	}
	if shaCLI != shaServidor {
		t.Errorf("las huellas no coinciden:\n  CLI      %s\n  servidor %s", shaCLI, shaServidor)
	}
}
