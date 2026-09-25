package mcp

import (
	"context"
	"io"
	"net"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"musubi/internal/config"
	"musubi/internal/embedding"
)

// REVOCAR SURTE EFECTO SIN REINICIAR, EN EL SERVIDOR DE VERDAD.
//
// `musubi token revoke` promete que el token deja de autenticar en ≤10 s sin reiniciar el cerebro,
// y lo que cumple esa promesa es UNA línea de ListenAndServeHTTP: `go reload.watch(ctx)`. Las otras
// pruebas de la recarga llaman a reloadIfChanged a mano, así que si esa línea se perdiera en un
// refactor del arranque la suite seguiría verde, el CLI seguiría diciendo «≤10 s» y el token
// revocado seguiría entrando hasta el próximo reinicio.
//
// Acá se arranca el servidor entero, como lo arranca musubi-brain, se revoca con la misma función
// que usa `musubi token revoke`, y se espera el 401. El intervalo se acorta para no esperar 10 s;
// lo que se mide es que ALGUIEN vigile el archivo, no cada cuánto.
//
// Sabotaje que la pone roja: no lanzar el watch del registro al arrancar.
// arnes: archivo="internal/mcp/http.go"
// arnes: de="\t\tgo reload.watch(ctx)\n"
// arnes: a="\t\t_ = reload\n"
func TestRevocarSurteEfectoSinReiniciarElServidor(t *testing.T) {
	anterior := principalsReloadInterval
	principalsReloadInterval = 20 * time.Millisecond
	t.Cleanup(func() { principalsReloadInterval = anterior })

	ruta := filepath.Join(t.TempDir(), "principals.yaml")
	writeRegAt(t, ruta, `principals:
  - name: queda
    token_sha256: `+hashToken("tok-queda")+`
    role: reader
    read: all
  - name: revocada
    token_sha256: `+hashToken("tok-revocada")+`
    role: reader
    read: all
`, time.Unix(6_000_000, 0))

	// Un puerto libre de loopback: ListenAndServeHTTP no devuelve el suyo, así que se pide uno al
	// sistema y se suelta justo antes de arrancar.
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("no conseguí un puerto de loopback: %v", err)
	}
	addr := l.Addr().String()
	_ = l.Close()

	s := newTestServer(t, embedding.NoopProvider{})
	ctx, cancel := context.WithCancel(context.Background())
	termino := make(chan error, 1)
	go func() {
		termino <- s.ListenAndServeHTTP(ctx, config.ServiceConfig{Addr: addr, PrincipalsFile: ruta, RequestTimeoutSeconds: 10})
	}()
	// Se apaga ANTES de devolver el intervalo (las limpiezas corren al revés): así el servidor ya
	// terminó cuando la variable vuelve a su valor.
	t.Cleanup(func() {
		cancel()
		select {
		case err := <-termino:
			if err != nil {
				t.Errorf("ListenAndServeHTTP terminó con error: %v", err)
			}
		case <-time.After(10 * time.Second):
			t.Error("ListenAndServeHTTP no terminó al cancelar el contexto")
		}
	})

	cliente := &http.Client{Timeout: 5 * time.Second}
	estado := func(bearer string) int {
		req, _ := http.NewRequest(http.MethodGet, "http://"+addr+"/metrics", nil)
		req.Header.Set("Authorization", "Bearer "+bearer)
		resp, err := cliente.Do(req)
		if err != nil {
			return 0
		}
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
		return resp.StatusCode
	}
	esperar := func(plazo time.Duration, bearer string, quiere int) int {
		t.Helper()
		limite := time.Now().Add(plazo)
		for {
			got := estado(bearer)
			if got == quiere || time.Now().After(limite) {
				return got
			}
			select {
			case err := <-termino:
				termino <- err // lo devuelve para que la limpieza no espere uno que ya llegó
				t.Fatalf("ListenAndServeHTTP terminó antes de tiempo: %v", err)
			case <-time.After(20 * time.Millisecond):
			}
		}
	}

	// El control: el servidor escucha y la credencial todavía vigente entra. Sin esto, un 401 de
	// abajo podría ser un servidor que nunca la aceptó.
	if got := esperar(10*time.Second, "tok-revocada", http.StatusOK); got != http.StatusOK {
		t.Fatalf("antes de revocar, /metrics con la credencial vigente da %d: la prueba no mediría nada", got)
	}

	// Lo mismo que hace `musubi token revoke`: sacar la línea del archivo, sin tocar el proceso.
	if ok, err := RemovePrincipal(ruta, "revocada"); err != nil || !ok {
		t.Fatalf("RemovePrincipal = %v, %v", ok, err)
	}

	if got := esperar(5*time.Second, "tok-revocada", http.StatusUnauthorized); got != http.StatusUnauthorized {
		t.Fatalf("cinco segundos después de revocarla, la credencial todavía da %d en /metrics: el "+
			"servidor no vigila principals.yaml, y el token revocado sigue entrando hasta el próximo "+
			"reinicio aunque `musubi token revoke` diga «≤10 s, sin reiniciar»", got)
	}
	// Y la que quedó sigue entrando: la recarga tomó el archivo nuevo, no tiró el registro.
	if got := estado("tok-queda"); got != http.StatusOK {
		t.Errorf("después de la recarga, la credencial que NO se revocó da %d", got)
	}
}
