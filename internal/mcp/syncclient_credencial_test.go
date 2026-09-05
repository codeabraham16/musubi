package mcp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"musubi/internal/config"
)

func cfgSyncDePrueba(tokenEnv string) config.SyncConfig {
	return config.SyncConfig{
		Enabled:            true,
		CentralURL:         "http://127.0.0.1:7717",
		AuthTokenEnv:       tokenEnv,
		AllowInsecureToken: true,
	}
}

// El fallo que costó horas el 2026-09-05: auth_token_env declarado, variable vacía, y el cliente
// se construía igual. Salía a la red sin cabecera Authorization y el central contestaba 401 en
// cada drain, con la configuración «puesta». Ahora no se construye.
func TestNewSyncClientNoSeConstruyeSinCredencialCuandoLaConfigLaNombra(t *testing.T) {
	os.Unsetenv("PRUEBA_SYNC_TOKEN")
	os.Unsetenv("PRUEBA_SYNC_TOKEN_FILE")

	c, err := NewSyncClient(cfgSyncDePrueba("PRUEBA_SYNC_TOKEN"))
	if err == nil {
		t.Fatalf("se esperaba error: un cliente sin credencial es una tormenta de 401 en silencio (obtuve %#v)", c)
	}
	// El mensaje tiene que nombrar LAS DOS formas, o quien lo lea prueba sólo una.
	if !strings.Contains(err.Error(), "PRUEBA_SYNC_TOKEN_FILE") {
		t.Fatalf("el error tiene que nombrar también la variante _FILE, dijo: %v", err)
	}
}

func TestNewSyncClientAceptaLaCredencialPorArchivo(t *testing.T) {
	ruta := filepath.Join(t.TempDir(), "token")
	if err := os.WriteFile(ruta, []byte("msb_desde-archivo\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	os.Unsetenv("PRUEBA_SYNC_TOKEN")
	t.Setenv("PRUEBA_SYNC_TOKEN_FILE", ruta)

	c, err := NewSyncClient(cfgSyncDePrueba("PRUEBA_SYNC_TOKEN"))
	if err != nil {
		t.Fatalf("con el archivo puesto tenía que construirse: %v", err)
	}
	if c.token != "msb_desde-archivo" {
		t.Fatalf("token mal resuelto: %q", c.token)
	}
}

// Sin auth_token_env no hay nada que exigir: hay centrales que no autentican.
func TestNewSyncClientSinAuthTokenEnvSigueSiendoValido(t *testing.T) {
	if _, err := NewSyncClient(cfgSyncDePrueba("")); err != nil {
		t.Fatalf("sin auth_token_env no debería fallar: %v", err)
	}
}
