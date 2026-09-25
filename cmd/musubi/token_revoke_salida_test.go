package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"musubi/internal/mcp"
)

// `musubi token revoke` NO PIDE REINICIAR EL CEREBRO.
//
// Lo pedía («Reiniciá musubi-brain.service para aplicar.») y era falso y caro: el cerebro relee
// principals.yaml solo cada 10 s, y reiniciar el central corta el sync de todas las máquinas para
// aplicar algo que ya se aplicaba solo. El mensaje nuevo lo dice y nombra los dos casos en que la
// relectura no alcanza. Que la orden de reiniciar vuelva es la regresión que custodia esta prueba;
// que la promesa del mensaje sea cierta la custodia TestRevocarSurteEfectoSinReiniciarElServidor,
// en internal/mcp, contra el servidor entero.
//
// Sabotaje que la pone roja: volver a pedir el reinicio (se AGREGA lo prohibido).
// arnes: archivo="cmd/musubi/token.go"
// arnes: de="sin reiniciar.\\n\", *name)"
// arnes: a="sin reiniciar. Reiniciá musubi-brain.service para aplicar.\\n\", *name)"
func TestTokenRevokeNoPideReiniciarElCerebro(t *testing.T) {
	ruta := filepath.Join(t.TempDir(), "principals.yaml")
	if err := os.WriteFile(ruta, []byte(`principals:
  - name: vieja
    token_sha256: `+strings.Repeat("a", 64)+`
    project_id: casa
    role: reader
  - name: queda
    token_sha256: `+strings.Repeat("b", 64)+`
    project_id: casa
    role: reader
`), 0o600); err != nil {
		t.Fatal(err)
	}

	salida := capturarSalida(t, func() { tokenRevoke([]string{"--name", "vieja", "--file", ruta}) })

	// El control: la revocación pasó de verdad. Sin esto, una salida vacía daría el verde de abajo.
	if !strings.Contains(salida, `"vieja" revocado`) {
		t.Fatalf("token revoke no dijo que revocó a \"vieja\":\n%s", salida)
	}
	infos, err := mcp.ListPrincipalsInfo(ruta)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range infos {
		if p.Name == "vieja" {
			t.Fatalf("token revoke dijo que revocó a \"vieja\" y sigue en el archivo:\n%s", salida)
		}
	}

	primera, _, _ := strings.Cut(salida, "\n")
	if !strings.Contains(primera, "sin reiniciar") {
		t.Errorf("la primera línea de token revoke no dice que se aplica sin reiniciar:\n  %s", primera)
	}
	if strings.Contains(strings.ToLower(salida), "reiniciá") {
		t.Errorf("token revoke vuelve a mandar a reiniciar el cerebro, y el cerebro ya relee el archivo "+
			"solo en ≤10 s: un reinicio del central corta el sync de todas las máquinas por nada.\n%s", salida)
	}
}
