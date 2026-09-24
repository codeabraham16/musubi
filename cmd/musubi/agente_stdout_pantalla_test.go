package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"musubi/internal/fleet"
)

// LO QUE CUSTODIA: que la línea que el AGENTE imprime por cada comando que atiende no lleve la
// contraseña de una sesión de pantalla. Es la superficie que faltaba: `TestElArgvDeBitacora…`
// promete «en ninguna superficie» y miraba sólo las del cerebro.
//
// DEFECTO VIVO, NO HIPOTÉTICO. Lo encontró la auditoría A131 (ronda 8, 2026-09-23) sin mutar
// nada: `atenderComandos` imprimía `› ejecutando: musubi:pantalla <sesión> <CONTRASEÑA> <ttl>`.
// En Linux ese stdout es el journal de `musubi-agente`, persistente. EXPOSICIÓN MEDIDA ese día:
// en toda la historia hubo 4 sesiones, las 4 en `gio` (Windows), vencidas hacía más de 18 días;
// ahí el lanzador corre el agente en una consola OCULTA sin redirigir, así que las 4 líneas no
// quedaron en ningún archivo. El cerebro las guarda tapadas (4 de 4 con `[oculto]`).
//
// POR QUÉ UNA GUARDA SOBRE EL STDOUT Y NO SÓLO SOBRE LA FUNCIÓN. `ResumenArgv` ya tapa por sí
// mismo, y lo cuida su propia prueba en internal/fleet. Lo que ésa no ve es el OTRO camino: que
// este llamador vuelva a unir el argv a mano. Se corre el `atenderComandos` real y se lee lo que
// de verdad sale por stdout.
//
// El argv va con TRES partes a propósito: `aplicarSesionPantalla` lo rechaza por mal formado
// ANTES de tocar RustDesk, así que la prueba no cambia ninguna contraseña real de la máquina que
// la corre. La línea culpable se imprime antes de despachar, así que la forma del argv no la
// cambia. El resultado se reporta a un httptest local: ninguna máquina real.
//
// Sabotaje: que el agente una el argv a mano en vez de pasar por ResumenArgv.
// arnes: archivo="cmd/musubi/agent.go"
// arnes: de="cDim(\"›\"), fleet.ResumenArgv(c.Argv))"
// arnes: a="cDim(\"›\"), strings.Join(c.Argv, \" \"))"
func TestElAgenteNoImprimeLaContrasenaDePantalla(t *testing.T) {
	const secreto = "ClaveDePruebaQueNoPuedeSalir-9f3a"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	viejo := os.Stdout
	os.Stdout = w
	atenderComandos(srv.URL, "tok", []comandoRecibido{{
		ID: "cmd-1", Argv: []string{fleet.OpPantalla, "ses-42", secreto}, TimeoutSeg: 5,
	}})
	os.Stdout = viejo
	w.Close()
	salida, _ := io.ReadAll(r)

	// CONTROL: sin la línea «ejecutando», la prueba no habría capturado nada y pasaría en verde
	// sin haber mirado el stdout.
	if !strings.Contains(string(salida), "ejecutando") {
		t.Fatalf("no se capturó la línea que el agente imprime por comando; esta prueba no midió nada: %q", salida)
	}
	if strings.Contains(string(salida), secreto) {
		t.Errorf("EL AGENTE IMPRIME LA CONTRASEÑA DE PANTALLA en su stdout —en Linux, el journal "+
			"de `musubi-agente`—: %q", salida)
	}
	if !strings.Contains(string(salida), "ses-42") {
		t.Errorf("la línea perdió el id de sesión, que es lo que sirve para cruzarla con la bitácora: %q", salida)
	}
}
