package mcp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// UN TOKEN QUE VALE PARA SIEMPRE ES LO PRIMERO QUE MIRA UN AUDITOR.
//
// Ola 0 del plan empresa. Hasta hoy no existía vencimiento en el registro: una credencial filtrada
// hace tres meses seguía abriendo la puerta, y la única forma de cerrarla era que alguien se
// acordara de borrar la línea a mano.
//
// El reloj entra por un seam (`ahoraParaVencimiento`) y no por fechas escritas a mano: una prueba
// anclada a 2027 empieza a fallar sola ese año y se borra en vez de arreglarse.
//
// Sabotaje que la hace fallar: sacar el `if match.Vencida(...)` de resolve.
func TestUnPrincipalVencidoNoAutentica(t *testing.T) {
	const tok = "token-que-vence"
	reg := registroConExpires(t, `principals:
  - name: temporal
    token_sha256: `+hashToken(tok)+`
    project_id: casa
    role: reader
    expires: "2026-06-01T00:00:00Z"
`)
	anterior := ahoraParaVencimiento
	t.Cleanup(func() { ahoraParaVencimiento = anterior })

	// ANTES del vencimiento: autentica.
	ahoraParaVencimiento = func() time.Time { return time.Date(2026, 5, 31, 23, 0, 0, 0, time.UTC) }
	if p, ok := reg.resolve(tok); !ok || p == nil {
		t.Fatal("la credencial no vencida no autenticó: el vencimiento se está aplicando al revés")
	}

	// DESPUÉS: no autentica.
	ahoraParaVencimiento = func() time.Time { return time.Date(2026, 6, 1, 0, 0, 1, 0, time.UTC) }
	if _, ok := reg.resolve(tok); ok {
		t.Error("una credencial VENCIDA sigue autenticando: un token filtrado hace meses abre la puerta igual")
	}

	// EXACTAMENTE en la fecha: vencida. El borde va para el lado seguro, que es el único que se
	// puede defender en una auditoría.
	ahoraParaVencimiento = func() time.Time { return time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC) }
	if _, ok := reg.resolve(tok); ok {
		t.Error("justo en el instante del vencimiento la credencial todavía vale: el borde tiene que ir para el lado seguro")
	}
}

// SIN `expires:` NADA CAMBIA. Es la mitad que hace que estrenar esto no le rompa la configuración
// a nadie: todo principals.yaml que ya existe sigue significando exactamente lo mismo.
//
// Sabotaje: que parsearVencimiento devuelva time.Now() ante un valor vacío.
func TestSinExpiresLaCredencialNoVence(t *testing.T) {
	const tok = "token-eterno"
	reg := registroConExpires(t, `principals:
  - name: de-siempre
    token_sha256: `+hashToken(tok)+`
    project_id: casa
    role: reader
`)
	anterior := ahoraParaVencimiento
	t.Cleanup(func() { ahoraParaVencimiento = anterior })
	ahoraParaVencimiento = func() time.Time { return time.Date(2099, 1, 1, 0, 0, 0, 0, time.UTC) }

	if _, ok := reg.resolve(tok); !ok {
		t.Error("un principal SIN expires dejó de autenticar: estrenar el vencimiento le rompió la configuración a todo el mundo")
	}
}

// UNA FECHA ILEGIBLE ES UN ERROR DE ARRANQUE, NO UNA CREDENCIAL ETERNA.
//
// Es el tramo que más importa y el más fácil de escribir al revés. Si un `expires` con un typo se
// ignorara en silencio, el resultado sería exactamente lo contrario de lo que quiso quien lo
// escribió: pidió una credencial que venza y se quedó con una que no vence nunca. El modo de falla
// de un typo tiene que ser que el cerebro no arranque.
//
// Sabotaje: que parsearVencimiento devuelva (time.Time{}, nil) cuando time.Parse falla.
func TestUnExpiresIlegibleImpideArrancar(t *testing.T) {
	dir := t.TempDir()
	ruta := filepath.Join(dir, "principals.yaml")
	if err := os.WriteFile(ruta, []byte(`principals:
  - name: con-typo
    token_sha256: `+hashToken("x")+`
    project_id: casa
    role: reader
    expires: "el jueves que viene"
`), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := loadPrincipals(ruta, "")
	if err == nil {
		t.Fatal("un `expires` ilegible se aceptó: el typo produjo una credencial ETERNA, que es lo contrario de lo que pidió quien lo escribió")
	}
	if !strings.Contains(err.Error(), "con-typo") || !strings.Contains(err.Error(), "RFC3339") {
		t.Errorf("el error no dice QUIÉN ni QUÉ formato se esperaba: %v", err)
	}
}

func registroConExpires(t *testing.T, yaml string) *PrincipalRegistry {
	t.Helper()
	ruta := filepath.Join(t.TempDir(), "principals.yaml")
	if err := os.WriteFile(ruta, []byte(yaml), 0o600); err != nil {
		t.Fatal(err)
	}
	reg, err := loadPrincipals(ruta, "")
	if err != nil {
		t.Fatalf("no se pudo cargar el registro de prueba: %v", err)
	}
	return reg
}
