package mcp

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

// LA PRUEBA DEL AGUJERO (S8, boca 2 de 2): el MAPA que devuelve el tool `musubi_token_list` tiene
// que decir el vencimiento.
//
// Es la hermana de TestLaFilaDeTokenListDiceQueLaCredencialVencio (cmd/musubi): las dos únicas
// bocas por las que el vencimiento sale del cerebro. Borrar las claves "expires" y "vencimiento"
// de este mapa compilaba y dejaba en verde los paquetes enteros, porque lo único custodiado era
// ListPrincipalsInfo —la capa de ABAJO—, que puede seguir resolviendo `VENCIDA` perfecto mientras
// esta boca la tira. Desde afuera el resultado es el mismo: un agente admin lista principals, ve
// permisos, y no tiene cómo saber cuál ya no ejecuta.
//
// Cómo está anclada:
//
//   - Ejercita el tool DE VERDAD (s.toolTokenList con un ctx admin) y parsea su JSON, no la
//     función de abajo ni el struct intermedio.
//   - Lo esperado se DERIVA de ListPrincipalsInfo sobre el mismo registro: nada tipeado.
//   - La PRESENCIA de cada clave se chequea aparte del valor, porque el sabotaje medido fue
//     borrarlas: una clave ausente se lee como "" en Go y "" es «no sé» disfrazado de «medí».
//   - Y hay una propiedad que no mira nombres de clave: dos principals en estados distintos no
//     pueden devolver el MISMO objeto sacando el nombre. Eso mata también la forma futura del
//     bug (devolver siempre la misma constante, o renombrar la clave a una que nadie lee).
func TestElToolTokenListDiceQueLaCredencialVencio(t *testing.T) {
	relojDeVencimiento(t, time.Date(2026, 9, 10, 0, 0, 0, 0, time.UTC))

	ruta := filepath.Join(t.TempDir(), "principals.yaml")
	if err := os.WriteFile(ruta, []byte(`principals:
  - name: contratista
    token_sha256: `+hashToken("a")+`
    project_id: casa
    role: reader
    expires: "2020-01-01T00:00:00Z"
  - name: de-siempre
    token_sha256: `+hashToken("b")+`
    project_id: casa
    role: reader
  - name: renovado
    token_sha256: `+hashToken("c")+`
    project_id: casa
    role: reader
    expires: "2030-01-01T00:00:00Z"
  - name: con-typo
    token_sha256: `+hashToken("d")+`
    project_id: casa
    role: reader
    expires: "el jueves que viene"
`), 0o600); err != nil {
		t.Fatal(err)
	}

	esperado := map[string]PrincipalInfo{}
	estados := map[string]bool{}
	infos, err := ListPrincipalsInfo(ruta)
	if err != nil {
		t.Fatalf("ListPrincipalsInfo: %v", err)
	}
	for _, p := range infos {
		esperado[p.Name] = p
		estados[p.Vencimiento] = true
	}
	// Si el registro dejó de producir los cuatro estados, esta prueba no está midiendo el
	// vencimiento: tiene que fallar, no pasar en silencio.
	if len(esperado) != 4 || len(estados) != 4 {
		t.Fatalf("el registro de prueba tiene que dar 4 principals con 4 estados distintos; dio %d principals y %d estados (%v)",
			len(esperado), len(estados), estados)
	}

	s := &McpServer{principalsFile: ruta}
	ctx := withPrincipal(context.Background(), &Principal{Name: "jefa", Role: RoleAdmin, ProjectID: "casa"})
	res, rpcErr := s.toolTokenList(ctx, nil)
	if rpcErr != nil {
		t.Fatalf("toolTokenList: %v", rpcErr)
	}
	resp, ok := res.(CallToolResponse)
	if !ok || len(resp.Content) == 0 {
		t.Fatalf("toolTokenList devolvió %T sin contenido: %#v", res, res)
	}
	var payload struct {
		Principals []map[string]interface{} `json:"principals"`
	}
	if err := json.Unmarshal([]byte(resp.Content[0].Text), &payload); err != nil {
		t.Fatalf("el resultado del tool no es JSON parseable (%v): %s", err, resp.Content[0].Text)
	}

	devuelto := map[string]map[string]interface{}{}
	for _, p := range payload.Principals {
		nombre, _ := p["name"].(string)
		if _, esPrincipal := esperado[nombre]; !esPrincipal {
			t.Fatalf("el tool devolvió un principal desconocido %q: %v", nombre, p)
		}
		devuelto[nombre] = p
	}
	if len(devuelto) != len(esperado) {
		t.Fatalf("el tool devolvió %d principals distintos y el registro tiene %d", len(devuelto), len(esperado))
	}

	for nombre, info := range esperado {
		obj := devuelto[nombre]
		// LA CLAVE TIENE QUE ESTAR. Ausente se leería como "" y "" significaría «no vence»: una
		// credencial muerta contestada como sana.
		crudo, hayExpires := obj["expires"]
		if !hayExpires {
			t.Errorf("%s: el tool no devuelve la clave \"expires\": el vencimiento es invisible para quien lista", nombre)
		} else if crudo != info.Expires {
			t.Errorf("%s: expires=%v y el registro dice %q", nombre, crudo, info.Expires)
		}
		estado, hayEstado := obj["vencimiento"]
		if !hayEstado {
			t.Errorf("%s: el tool no devuelve la clave \"vencimiento\": quien lista tendría que resolver la fecha contra el reloj de memoria", nombre)
		} else if estado != info.Vencimiento {
			t.Errorf("%s: vencimiento=%v y tendría que ser %q", nombre, estado, info.Vencimiento)
		} else if estado == "" {
			t.Errorf("%s: vencimiento vacío — un vacío acá es «no pude medir» leído como «medí y está bien»", nombre)
		}
	}

	// LA PROPIEDAD, SIN MIRAR NOMBRES DE CLAVE: dos credenciales en estados distintos no pueden
	// devolver el mismo objeto sacando el nombre. Con las dos claves borradas, la VENCIDA y la
	// vigente son objetos idénticos, que es la definición operativa del defecto.
	for a, ia := range esperado {
		for b, ib := range esperado {
			if a >= b || ia.Vencimiento == ib.Vencimiento {
				continue
			}
			if reflect.DeepEqual(sinNombreDelPrincipal(devuelto[a]), sinNombreDelPrincipal(devuelto[b])) {
				t.Errorf("%s (%s) y %s (%s) se devuelven IDÉNTICOS sacando el nombre (%v): el tool no distingue una credencial muerta de una viva",
					a, ia.Vencimiento, b, ib.Vencimiento, sinNombreDelPrincipal(devuelto[a]))
			}
		}
	}
}

// sinNombreDelPrincipal saca las claves de identidad, que son lo único que siempre diferencia a
// dos principals, para que la comparación mida si queda ALGO que distinga sus estados.
func sinNombreDelPrincipal(obj map[string]interface{}) map[string]interface{} {
	copia := map[string]interface{}{}
	for k, v := range obj {
		if k == "name" {
			continue
		}
		copia[k] = v
	}
	return copia
}
