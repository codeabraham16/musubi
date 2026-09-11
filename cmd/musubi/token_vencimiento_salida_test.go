package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"musubi/internal/mcp"
)

// LA PRUEBA DEL AGUJERO (S8, boca 1 de 2): la fila que IMPRIME `musubi token list` tiene que
// decir el vencimiento.
//
// El vencimiento ya estaba resuelto y custodiado una capa más abajo (ListPrincipalsInfo, con
// TestElListadoDePrincipalsDiceQueLaCredencialVencio). Eso no alcanza: la capa de abajo puede
// seguir devolviendo `VENCIDA` mientras el CLI la tira a la basura con un `vence := ""`, y ahí el
// operador ve un principal con permisos sin ninguna forma de saber que ya no ejecuta. Ese
// sabotaje —una línea— compilaba y dejaba en verde los dos paquetes enteros.
//
// Por qué mira el STDOUT y no la función de abajo: lo que se custodia es LO QUE EL OPERADOR VE.
// La única superficie donde el vencimiento existe para él es esta fila.
//
// Cómo está anclada, para que no la satisfaga un vecino:
//
//   - Lo esperado NO está tipeado: se DERIVA de ListPrincipalsInfo sobre el mismo registro. Si
//     mañana cambian los nombres de los estados, la guarda sigue midiendo lo mismo.
//   - Se busca LA FILA de cada principal (primer campo == su nombre), no el texto suelto en la
//     salida: el encabezado, un mensaje o la fila del vecino no la satisfacen.
//   - Y además de la contención hay una prueba que NO mira texto: dos principals con estados
//     distintos tienen que imprimir filas DISTINTAS (sacando el nombre). Ésa es la propiedad que
//     de verdad importa —que el operador pueda distinguir una muerta de una viva— y es la que
//     mata cualquier forma futura del bug, incluida «imprimir siempre la misma constante».
func TestLaFilaDeTokenListDiceQueLaCredencialVencio(t *testing.T) {
	// Fechas absurdamente lejos del presente en las dos direcciones: la guarda no depende del
	// reloj de la máquina ni de ningún seam (el del registro vive en otro paquete).
	ruta := filepath.Join(t.TempDir(), "principals.yaml")
	if err := os.WriteFile(ruta, []byte(`principals:
  - name: contratista
    token_sha256: aaaa
    project_id: casa
    role: reader
    expires: "2001-02-03T04:05:06Z"
  - name: de-siempre
    token_sha256: bbbb
    project_id: casa
    role: reader
  - name: renovado
    token_sha256: cccc
    project_id: casa
    role: reader
    expires: "2999-02-03T04:05:06Z"
  - name: con-typo
    token_sha256: dddd
    project_id: casa
    role: reader
    expires: "el jueves que viene"
`), 0o600); err != nil {
		t.Fatal(err)
	}

	// Lo esperado sale de la capa de abajo, no de un literal.
	infos, err := mcp.ListPrincipalsInfo(ruta)
	if err != nil {
		t.Fatalf("ListPrincipalsInfo: %v", err)
	}
	esperado := map[string]mcp.PrincipalInfo{}
	estados := map[string]bool{}
	for _, p := range infos {
		esperado[p.Name] = p
		estados[p.Vencimiento] = true
	}
	// UN CERO NO PUEDE SIGNIFICAR «NO PUDE MEDIR»: si el registro dejó de producir los cuatro
	// estados, esta prueba no está ejercitando el vencimiento y tiene que decirlo, no pasar.
	if len(esperado) != 4 || len(estados) != 4 {
		t.Fatalf("el registro de prueba tiene que dar 4 principals con 4 estados distintos; dio %d principals y %d estados (%v)",
			len(esperado), len(estados), estados)
	}

	salida := salidaDeTokenList(t, ruta)

	filas := map[string]string{}
	for _, linea := range strings.Split(salida, "\n") {
		campos := strings.Fields(linea)
		if len(campos) == 0 {
			continue
		}
		if _, esPrincipal := esperado[campos[0]]; !esPrincipal {
			continue
		}
		if anterior, repetida := filas[campos[0]]; repetida {
			t.Fatalf("%s aparece en dos filas (%q y %q): la guarda no sabría cuál mirar", campos[0], anterior, linea)
		}
		filas[campos[0]] = linea
	}

	for nombre, info := range esperado {
		fila, hay := filas[nombre]
		if !hay {
			t.Fatalf("`token list` no imprimió ninguna fila para %q; la salida fue:\n%s", nombre, salida)
		}
		// EL ESTADO, resuelto contra el reloj: que el operador no tenga que hacer la cuenta.
		if !strings.Contains(fila, info.Vencimiento) {
			t.Errorf("la fila de %s no dice el vencimiento %q — un operador la ve idéntica a una viva:\n  %s",
				nombre, info.Vencimiento, fila)
		}
		// Y LA FECHA CRUDA cuando la hay: «VENCIDA» sin fecha no deja auditar cuándo murió.
		if info.Expires != "" && !strings.Contains(fila, info.Expires) {
			t.Errorf("la fila de %s no dice la fecha %q del registro:\n  %s", nombre, info.Expires, fila)
		}
	}

	// LA PROPIEDAD, SIN MIRAR TEXTO: dos credenciales en estados distintos no pueden imprimirse
	// igual. Con `vence := ""` la fila de la VENCIDA y la de la vigente son la misma cadena
	// sacando el nombre, y eso es exactamente «el vencimiento es invisible desde afuera».
	for a, ia := range esperado {
		for b, ib := range esperado {
			if a >= b || ia.Vencimiento == ib.Vencimiento {
				continue
			}
			if sinNombre(filas[a], a) == sinNombre(filas[b], b) {
				t.Errorf("%s (%s) y %s (%s) imprimen la MISMA fila sacando el nombre (%q): el listado no distingue una credencial muerta de una viva",
					a, ia.Vencimiento, b, ib.Vencimiento, sinNombre(filas[a], a))
			}
		}
	}
}

// sinNombre normaliza una fila para compararla con otra: colapsa los espacios de relleno de las
// columnas y saca el nombre del principal, que es lo ÚNICO que siempre las diferencia.
func sinNombre(fila, nombre string) string {
	return strings.TrimSpace(strings.ReplaceAll(strings.Join(strings.Fields(fila), " "), nombre, ""))
}

// salidaDeTokenList corre `musubi token list --file <ruta>` y devuelve lo que imprimió por stdout.
func salidaDeTokenList(t *testing.T, ruta string) string {
	t.Helper()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("no se pudo abrir el pipe para capturar la salida: %v", err)
	}
	stdoutAnterior := os.Stdout
	os.Stdout = w

	// El lector arranca ANTES y en su propia goroutine: el pipe tiene buffer acotado.
	hecho := make(chan string, 1)
	go func() {
		var b strings.Builder
		_, _ = io.Copy(&b, r)
		hecho <- b.String()
	}()

	tokenList([]string{"--file", ruta})

	os.Stdout = stdoutAnterior
	_ = w.Close()
	salida := <-hecho
	_ = r.Close()
	return salida
}
