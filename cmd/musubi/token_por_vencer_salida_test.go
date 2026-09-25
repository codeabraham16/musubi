package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"musubi/internal/mcp"
)

// LA FILA DE UNA CREDENCIAL «POR VENCER» DICE CUÁNTOS DÍAS LE QUEDAN.
//
// Es la fila que abre quien llega por la alerta `CredencialPorVencer`, y «por vencer
// (2026-09-27T11:00:00Z)» le deja la resta de cabeza justo a quien está apurado.
//
// El reloj del registro vive en otro paquete y no se puede fijar desde acá, así que las fechas
// salen del reloj real con margen de horas para los dos lados: 71 h son 3 días redondeando hacia
// arriba corra a la hora que corra. Y lo esperado se DERIVA de ListPrincipalsInfo, igual que en
// TestLaFilaDeTokenListDiceQueLaCredencialVencio: lo que se custodia es que el CLI no tire lo
// que la capa de abajo ya resolvió.
//
// Sabotaje que la pone roja: no agregar los días a la fila.
// arnes: archivo="cmd/musubi/token.go"
// arnes: de="\t\t\tvence += \", \" + faltanDias(p.DiasParaVencer)\n"
// arnes: a="\t\t\t_ = faltanDias(p.DiasParaVencer)\n"
func TestLaFilaDeTokenListDiceCuantoLeFalta(t *testing.T) {
	ahora := time.Now().UTC()
	ruta := filepath.Join(t.TempDir(), "principals.yaml")
	if err := os.WriteFile(ruta, []byte(`principals:
  - name: por-vencer
    token_sha256: aaaa
    project_id: casa
    role: reader
    expires: "`+ahora.Add(71*time.Hour).Format(time.RFC3339)+`"
  - name: lejana
    token_sha256: bbbb
    project_id: casa
    role: reader
    expires: "2999-02-03T04:05:06Z"
`), 0o600); err != nil {
		t.Fatal(err)
	}

	infos, err := mcp.ListPrincipalsInfo(ruta)
	if err != nil {
		t.Fatalf("ListPrincipalsInfo: %v", err)
	}
	esperado := map[string]mcp.PrincipalInfo{}
	for _, p := range infos {
		esperado[p.Name] = p
	}
	// El control: si la capa de abajo no la marca «por vencer» con 3 días, esta prueba no está
	// midiendo la fila sino el reloj.
	if p := esperado["por-vencer"]; p.Vencimiento != mcp.VencimientoPorVencer || p.DiasParaVencer != 3 {
		t.Fatalf("ListPrincipalsInfo dice %q con %d días para una credencial a 71 h; se esperaba %q con 3",
			p.Vencimiento, p.DiasParaVencer, mcp.VencimientoPorVencer)
	}

	salida := salidaDeTokenList(t, ruta)
	filas := map[string]string{}
	for _, linea := range strings.Split(salida, "\n") {
		if campos := strings.Fields(linea); len(campos) > 0 {
			if _, es := esperado[campos[0]]; es {
				filas[campos[0]] = linea
			}
		}
	}

	fila := filas["por-vencer"]
	quiere := fmt.Sprintf("faltan %d días", esperado["por-vencer"].DiasParaVencer)
	if !strings.Contains(fila, mcp.VencimientoPorVencer) || !strings.Contains(fila, quiere) {
		t.Errorf("la fila de una credencial por vencer no dice %q ni %q:\n  %s\nsalida completa:\n%s",
			mcp.VencimientoPorVencer, quiere, fila, salida)
	}
	// Y una vigente lejana no lleva cuenta: «faltan 355000 días» es ruido que tapa a la que importa.
	if lejana := filas["lejana"]; strings.Contains(lejana, "falta") {
		t.Errorf("la fila de una credencial vigente lleva una cuenta de días que nadie pidió:\n  %s", lejana)
	}
}
