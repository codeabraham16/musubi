package mcp

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"
)

// EL LISTADO AVISA ANTES DE QUE VENZA, NO DESPUÉS.
//
// Hasta acá `musubi token list` tenía cuatro estados, y entre «vigente» y «VENCIDA» no había nada:
// una credencial a la que le quedaban tres horas se listaba igual que una a la que le quedaban
// tres años. El primer aviso era el sync de una máquina muerto.
//
// Todo se lee por ListPrincipalsInfo, que es lo que usan las dos bocas (el CLI y la tool), y con
// el reloj del registro fijo: una prueba de «faltan 3 días» contra el reloj real depende de a qué
// hora corre.
//
// Sabotaje que la pone roja: que estadoDeVencimiento no tenga la rama «por vencer».
// arnes: archivo="internal/mcp/principals_admin.go"
// arnes: de="\tif t.Sub(ahora) < umbralPorVencer {\n\t\treturn VencimientoPorVencer\n\t}"
// arnes: a="\tif false && t.Sub(ahora) < umbralPorVencer {\n\t\treturn VencimientoPorVencer\n\t}"
//
// Sabotaje que la pone roja: redondear los días hacia abajo (a la de doce horas le «faltan 0 días»).
// arnes: archivo="internal/mcp/principals_admin.go"
// arnes: de="\treturn int((t.Sub(ahoraParaVencimiento()) + dia - 1) / dia)"
// arnes: a="\treturn int(t.Sub(ahoraParaVencimiento()) / dia)"
func TestElListadoAvisaQueLaCredencialEstaPorVencer(t *testing.T) {
	ahora := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	relojDeVencimiento(t, ahora)

	fecha := func(d time.Duration) string { return ahora.Add(d).Format(time.RFC3339) }
	casos := []struct {
		nombre, expires, estado string
		dias                    int
	}{
		{"en-tres-dias", fecha(3 * 24 * time.Hour), VencimientoPorVencer, 3},
		// Doce horas son UN día que falta, no cero: «faltan 0 días» se lee como «ya está».
		{"en-doce-horas", fecha(12 * time.Hour), VencimientoPorVencer, 1},
		// El borde: justo en el umbral ya no es «por vencer». Es el mismo `<` que la alerta.
		{"justo-en-el-umbral", fecha(umbralPorVencer), VencimientoVigente, 0},
		{"en-un-mes", fecha(30 * 24 * time.Hour), VencimientoVigente, 0},
		{"vencida", fecha(-time.Second), VencimientoVencida, 0},
		{"de-siempre", "", VencimientoNoVence, 0},
	}

	var yaml strings.Builder
	yaml.WriteString("principals:\n")
	for i, c := range casos {
		yaml.WriteString("  - name: " + c.nombre + "\n")
		yaml.WriteString("    token_sha256: " + hashToken("t"+strconv.Itoa(i)) + "\n")
		yaml.WriteString("    project_id: casa\n    role: reader\n")
		if c.expires != "" {
			yaml.WriteString("    expires: \"" + c.expires + "\"\n")
		}
	}
	ruta := filepath.Join(t.TempDir(), "principals.yaml")
	if err := os.WriteFile(ruta, []byte(yaml.String()), 0o600); err != nil {
		t.Fatal(err)
	}

	infos, err := ListPrincipalsInfo(ruta)
	if err != nil {
		t.Fatalf("ListPrincipalsInfo: %v", err)
	}
	porNombre := map[string]PrincipalInfo{}
	for _, p := range infos {
		porNombre[p.Name] = p
	}
	if len(porNombre) != len(casos) {
		t.Fatalf("el registro tiene %d principals y el listado devolvió %d", len(casos), len(porNombre))
	}
	for _, c := range casos {
		got := porNombre[c.nombre]
		if got.Vencimiento != c.estado {
			t.Errorf("%s (expires %q): el listado dice %q y tendría que decir %q",
				c.nombre, c.expires, got.Vencimiento, c.estado)
		}
		if got.DiasParaVencer != c.dias {
			t.Errorf("%s: el listado dice que faltan %d días y faltan %d", c.nombre, got.DiasParaVencer, c.dias)
		}
	}
}

// EL AVISO Y EL LISTADO USAN EL MISMO NÚMERO.
//
// La alerta `CredencialPorVencer` manda a mirar `musubi token list`. Si la alerta dijera 14 días y
// el listado marcara «por vencer» desde los 7, quien llega por el aviso vería todo «vigente» y no
// encontraría cuál es — y si fuera al revés, el listado diría «por vencer» durante una semana sin
// que nada avisara. Los dos archivos son válidos por separado; por eso existe este cruce, igual que
// TestElUmbralDeVencimientoDejaTresCorridasDeMargen para el certificado.
//
// Sabotaje que la pone roja: bajar la alerta a 7 días dejando el listado en 14.
// arnes: archivo="deploy/musubi-alerts.yml"
// arnes: de="        expr: musubi_principal_next_expiry_seconds < 14 * 24 * 3600"
// arnes: a="        expr: musubi_principal_next_expiry_seconds < 7 * 24 * 3600"
//
// Sabotaje que la pone roja: bajar el umbral del listado a 7 días dejando la alerta en 14.
// arnes: archivo="internal/mcp/principals_admin.go"
// arnes: de="const umbralPorVencer = 14 * 24 * time.Hour"
// arnes: a="const umbralPorVencer = 7 * 24 * time.Hour"
func TestElUmbralDeLaAlertaEsElDelListado(t *testing.T) {
	reglas := leerDeploy(t, "musubi-alerts.yml")
	i := strings.Index(reglas, "- alert: CredencialPorVencer")
	if i < 0 {
		t.Fatal("no está la alerta `CredencialPorVencer`: sin ella la credencial vence sin aviso y lo " +
			"primero que se ve es el sync de una máquina muerto")
	}
	bloque := reglas[i:]
	if j := strings.Index(bloque[1:], "- alert:"); j > 0 {
		bloque = bloque[:j]
	}
	m := regexp.MustCompile(`musubi_principal_next_expiry_seconds\s*<\s*(\d+)\s*\*\s*24\s*\*\s*3600`).FindStringSubmatch(bloque)
	if m == nil {
		t.Fatalf("no pude leer el umbral en días de `CredencialPorVencer`. Se espera la forma "+
			"`musubi_principal_next_expiry_seconds < N * 24 * 3600`, que deja el número legible en "+
			"días y comparable con el del listado:\n%s", bloque)
	}
	dias, err := strconv.Atoi(m[1])
	if err != nil {
		t.Fatal(err)
	}
	if alerta := time.Duration(dias) * 24 * time.Hour; alerta != umbralPorVencer {
		t.Errorf("`CredencialPorVencer` avisa a los %d días y `musubi token list` marca «por vencer» "+
			"desde los %v: quien abra el listado por la alerta no va a encontrar la credencial que la "+
			"disparó. Tienen que ser el mismo número (umbralPorVencer, en principals_admin.go).",
			dias, umbralPorVencer)
	}
}
