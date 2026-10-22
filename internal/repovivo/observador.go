package repovivo

import (
	"context"
	"errors"
	"time"

	"musubi/internal/logx"
)

// observador es el bucle de la zona en vivo de UN checkout: cada tick captura el working tree y,
// SÓLO si cambió desde la última foto publicada, crea el commit huérfano y lo empuja. El dedupe
// por sha del árbol es lo que evita el push espurio: a ritmo real el working tree pasa quieto la
// mayoría de los ticks, y un push por tick saturaría la red y la ref sin ninguna foto nueva.
type observador struct {
	foto      *fotografo
	trans     *transporte
	intervalo time.Duration

	ultimoTree string // sha del último árbol PUBLICADO; "" hasta la primera foto que sube
}

func nuevoObservador(foto *fotografo, trans *transporte, intervalo time.Duration) *observador {
	return &observador{foto: foto, trans: trans, intervalo: intervalo}
}

// tick corre una pasada: captura el árbol, decide si hay algo nuevo, y si lo hay lo publica.
// Devuelve publico=true sólo cuando subió una foto nueva. Casos que NO son error y NO publican:
//   - una operación git en vuelo (rebase/merge/…): se saltea el tick (R7).
//   - el árbol no cambió desde la última foto publicada: dedupe.
//
// El árbol se escribe SIEMPRE (es barato y es el detector de cambio); el commit y el push —lo
// caro— sólo ocurren tras pasar el dedupe. ultimoTree se avanza SÓLO si el push salió bien, así
// un fallo de red se reintenta en el próximo tick en vez de perder la foto.
func (o *observador) tick(ctx context.Context) (publico bool, err error) {
	tree, err := o.foto.escribirTree(ctx)
	if err != nil {
		if errors.Is(err, errOperacionEnVuelo) {
			return false, nil // saltear este tick, no es falla
		}
		return false, err
	}
	if tree == o.ultimoTree {
		return false, nil // nada cambió desde la última foto publicada
	}
	sha, err := o.foto.commitDeTree(ctx, tree)
	if err != nil {
		return false, err
	}
	if err := o.trans.publicar(ctx, sha); err != nil {
		return false, err
	}
	o.ultimoTree = tree
	return true, nil
}

// correr corre el bucle hasta que ctx muera. Toma una foto inmediata al arrancar (no espera el
// primer intervalo) y después una por tick. Best-effort estilo hooks: un error de un tick se
// loguea CON FRENO (uno por minuto como mucho) y el bucle sigue; un working tree que no se puede
// fotografiar durante horas no puede llenar el stderr de la sesión.
func (o *observador) correr(ctx context.Context) {
	reloj := time.NewTicker(o.intervalo)
	defer reloj.Stop()

	var ultimoLog time.Time
	var fallos int
	// Freno exponencial ante fallos de publicación (B4c): un remoto caído o sin credencial no debe
	// hacer que CADA tick reintente el push —martillando la red (p.ej. 401 tras 401)— ni dejar un
	// commit huérfano por tick. Tras un fallo posterga el próximo intento duplicando la espera (hasta
	// un tope); un éxito lo resetea. Mientras frena, el tick se saltea entero: ni foto ni red.
	freno := nuevoBackoffRed(o.intervalo, 5*time.Minute)
	pasada := func(ahora time.Time) {
		if !freno.puedeIntentar(ahora) {
			return // en back-off tras un fallo: saltear este tick sin tocar la red
		}
		if _, err := o.tick(ctx); err != nil {
			freno.fallo(ahora)
			fallos++
			if time.Since(ultimoLog) > time.Minute {
				logx.Error("repo-vivo: no se pudo tomar/publicar la foto (se sigue intentando)",
					"ref", o.trans.ref, "fallos", fallos, "espera", freno.espera, "error", err)
				ultimoLog = time.Now()
				fallos = 0
			}
			return
		}
		freno.exito()
	}

	pasada(time.Now())
	for {
		select {
		case <-ctx.Done():
			return
		case t := <-reloj.C:
			pasada(t)
		}
	}
}

// backoffRed es el freno exponencial del observador ante fallos de publicación. NO lee el reloj por
// su cuenta —se le pasa `ahora`— para poder probarlo determinista. `espera` arranca en 0 (sin freno),
// salta a `base` en el primer fallo y se duplica en cada fallo siguiente hasta `tope`; un éxito la
// vuelve a 0. `hasta` es el instante antes del cual no se vuelve a intentar.
type backoffRed struct {
	base   time.Duration
	tope   time.Duration
	espera time.Duration
	hasta  time.Time
}

func nuevoBackoffRed(base, tope time.Duration) backoffRed {
	if base <= 0 {
		base = time.Second
	}
	if tope < base {
		tope = base
	}
	return backoffRed{base: base, tope: tope}
}

// puedeIntentar informa si en `ahora` ya pasó el freno (o si no hay freno activo).
func (b *backoffRed) puedeIntentar(ahora time.Time) bool {
	return !ahora.Before(b.hasta)
}

// exito limpia el freno: el siguiente fallo vuelve a arrancar en `base`.
func (b *backoffRed) exito() {
	b.espera = 0
	b.hasta = time.Time{}
}

// fallo crece la espera (base, luego ×2 hasta tope) y fija hasta cuándo frenar, contado desde `ahora`.
func (b *backoffRed) fallo(ahora time.Time) {
	if b.espera <= 0 {
		b.espera = b.base
	} else {
		b.espera *= 2
		if b.espera > b.tope {
			b.espera = b.tope
		}
	}
	b.hasta = ahora.Add(b.espera)
}
