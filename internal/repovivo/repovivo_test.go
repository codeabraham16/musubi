package repovivo

import (
	"context"
	"strings"
	"testing"
	"time"
)

// TestArrancarGatea cubre el CABLEADO de Arrancar: que el punto de ensamblado respete de verdad el
// veredicto de las guardas (B2 enabled=off y M7 guarda-github), sin lo cual los fixes de identidad.go
// y config.go podrían estar bien y aun así no gobernar el arranque real. Sólo ejercita los caminos
// que NO encienden (retornan al instante): el ctx igual lleva un timeout corto como red de seguridad,
// para que si un día el gate se rompe y Arrancar se mete en el bucle, el test falle por aserción en
// vez de colgarse (con el gate sano el timeout nunca se alcanza).
func TestArrancarGatea(t *testing.T) {
	// (1) enabled=off apaga AUN con team_mode y un remoto empresarial válido: no enciende, sin error.
	t.Run("off apaga aun con team y remoto", func(t *testing.T) {
		dir := repoEnVivo(t)
		runGit(t, dir, "remote", "add", "origin", "https://forgejo.local/u/r")
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		enc, razon, err := Arrancar(ctx, Opciones{Dir: dir, Remoto: "origin", Modo: "off", TeamMode: true})
		if err != nil {
			t.Fatalf("enabled=off no es un error: %v", err)
		}
		if enc {
			t.Error("enabled=off debía NO encender ni con team_mode y remoto empresarial")
		}
		if !strings.Contains(razon, "off") {
			t.Errorf("razón %q no menciona off", razon)
		}
	})

	// (2) la guarda DURA anti-github corta AUN con enabled=on: un remoto que resuelve a github.com no
	//     arranca (es el espejo público). Cablea M7 → debeEncender en el punto de ensamblado.
	t.Run("github corta aun con on", func(t *testing.T) {
		dir := repoEnVivo(t)
		runGit(t, dir, "remote", "add", "origin", "https://github.com/u/r.git")
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		enc, razon, err := Arrancar(ctx, Opciones{Dir: dir, Remoto: "origin", Modo: "on"})
		if err != nil {
			t.Fatalf("un remoto github no arranca, pero no es un error de construcción: %v", err)
		}
		if enc {
			t.Error("un remoto github.com debía NO arrancar ni con enabled=on (guarda dura del espejo)")
		}
		if !strings.Contains(razon, "github") {
			t.Errorf("razón %q no menciona github", razon)
		}
	})

	// (3) un Dir que no es un repo git: no enciende y SÍ devuelve error (algo que debería andar falló
	//     al construirse), que es el contrato de Arrancar distinto de "no debe encender".
	t.Run("no-repo da error", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		enc, _, err := Arrancar(ctx, Opciones{Dir: t.TempDir(), Remoto: "origin", Modo: "on"})
		if err == nil {
			t.Error("un Dir que no es repo git debía devolver error")
		}
		if enc {
			t.Error("no debía encender sobre un no-repo")
		}
	})
}

// TestArrancarEnciendeYPublica cubre el camino que TestArrancarGatea NO toca: el de Arrancar que SÍ
// enciende (repovivo.go:65-80). Es el único que ejercita el ENSAMBLADO completo —construir el fotógrafo,
// derivar la ref, construir el transporte y correr el bucle hasta publicar—; los caminos que no encienden
// retornan antes de armar nada. No publicamos nosotros ninguna foto: montamos un remoto bare local (NO
// github, así la guarda dura deja pasar), arrancamos en "on" y esperamos a que la PRIMERA foto automática
// aterrice sola bajo refs/vivo/<nodo>/<slug>. Que la ref aparezca prueba que el bucle corrió de punta a
// punta por el camino que enciende.
func TestArrancarEnciendeYPublica(t *testing.T) {
	_, dir, remoto, bare := repoConRemoto(t)

	// La ref que Arrancar va a derivar, calculada IGUAL que él: desde el toplevel real del checkout.
	// slugDeCheckout hashea ese path, así que si no coincidiera byte a byte el poll miraría una ref que
	// nunca se crea y el test expiraría por el motivo equivocado en vez de medir el push. gitSalida ya
	// trima, igual que el strings.TrimSpace de Arrancar. Pasamos un Nodo EXPLÍCITO (no el hostname) y lo
	// esperamos EN la ref: así el test también fija que Arrancar cablea o.Nodo hasta la ref —un
	// nodo(o.Nodo) cambiado por nodo("") publicaría bajo el hostname y el poll expiraría.
	toplevel := gitSalida(t, dir, "rev-parse", "--show-toplevel")
	ref := refDeCheckout(nodo("nodo-de-prueba"), slugDeCheckout(toplevel))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	type salida struct {
		enc   bool
		razon string
		err   error
	}
	hecho := make(chan salida, 1) // buffer 1: la goroutine nunca bloquea al mandar su resultado
	go func() {
		// Arrancar BLOQUEA en el bucle del observador hasta que ctx muera: corre en su propia goroutine.
		enc, razon, err := Arrancar(ctx, Opciones{Dir: dir, Remoto: remoto, Modo: "on", Intervalo: 20 * time.Millisecond, Nodo: "nodo-de-prueba"})
		hecho <- salida{enc, razon, err}
	}()

	// Esperamos a que la foto automática CREE la ref en el remoto. shaEnBare devuelve "" mientras no
	// exista, y es un proceso git aparte: no comparte memoria con la goroutine de Arrancar, así que -race
	// queda limpio. Si Arrancar vuelve antes de publicar, es que no encendió o no entró al bucle: falla.
	limite := time.After(10 * time.Second)
	tic := time.NewTicker(10 * time.Millisecond)
	defer tic.Stop()
	for shaEnBare(t, bare, ref) == "" {
		select {
		case <-limite:
			cancel()
			t.Fatalf("la foto automática no llegó a %s en 10s (¿Arrancar no encendió o no publicó?)", ref)
		case r := <-hecho:
			t.Fatalf("Arrancar volvió antes de publicar (enc=%v razon=%q err=%v): no entró al bucle", r.enc, r.razon, r.err)
		case <-tic.C:
		}
	}

	// La foto llegó: el camino que enciende corrió entero. Cerramos el bucle y cosechamos el retorno.
	cancel()
	select {
	case r := <-hecho:
		if !r.enc {
			t.Errorf("Arrancar debía reportar encendido=true, dio false (razón %q)", r.razon)
		}
		if r.err != nil {
			t.Errorf("el camino que enciende no debía dar error: %v", r.err)
		}
		if !strings.Contains(r.razon, "enabled = on") {
			t.Errorf("razón %q no refleja enabled=on", r.razon)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Arrancar no volvió tras cancelar el ctx: ¿el bucle ignora la cancelación?")
	}
}
