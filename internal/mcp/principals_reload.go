package mcp

import (
	"context"
	"os"
	"sync/atomic"
	"time"

	"musubi/internal/logx"
)

// principals_reload.go da RECARGA EN CALIENTE del registro de principals (Track 18). Sin esto
// loadPrincipals corría una sola vez al arranque, así que revocar o dar de alta a un miembro
// (editar principals.yaml) NO surtía efecto hasta reiniciar el daemon — una revocación que no es
// inmediata es un agujero: el token comprometido sigue autenticando hasta el próximo restart. Un
// goroutine de fondo vigila el mtime del archivo y re-lee cuando cambia. Es model-free y 0-deps
// (mtime-poll, no fsnotify), fiel al resto del repo.

// principalResolver abstrae "resolver un bearer a un principal": lo satisfacen el registro
// estático (*PrincipalRegistry, modo legacy sin archivo) y el recargable (*reloadableRegistry).
type principalResolver interface {
	resolve(token string) (*Principal, bool)
}

// principalsReloadInterval es cada cuánto se chequea el mtime del registro. 10s da una revocación
// casi-inmediata sin costo perceptible (un os.Stat por intervalo).
const principalsReloadInterval = 10 * time.Second

// reloadableRegistry envuelve el registro con recarga en caliente por mtime. El snapshot vigente
// vive en un atomic.Pointer (lectura lock-free desde cada request); solo el goroutine de watch lo
// reemplaza. Una recarga fallida (archivo a medio editar / malformado) CONSERVA el snapshot vigente
// (fail-safe: un typo transitorio no deja al equipo afuera) y se loguea.
type reloadableRegistry struct {
	path        string
	legacyToken string
	cur         atomic.Pointer[PrincipalRegistry]
	lastModNano int64 // mtime del último cargado; solo lo toca el goroutine de watch (sin carrera)
}

// newReloadableRegistry crea el envoltorio sembrado con el registro ya cargado y su mtime.
func newReloadableRegistry(path, legacyToken string, initial *PrincipalRegistry, initialMod time.Time) *reloadableRegistry {
	rr := &reloadableRegistry{path: path, legacyToken: legacyToken, lastModNano: initialMod.UnixNano()}
	rr.cur.Store(initial)
	return rr
}

// resolve autentica contra el snapshot vigente (lock-free).
func (rr *reloadableRegistry) resolve(token string) (*Principal, bool) {
	reg := rr.cur.Load()
	if reg == nil {
		return nil, false
	}
	return reg.resolve(token)
}

// watch re-lee el registro cuando cambia el mtime, hasta que ctx se cancela (shutdown del server).
func (rr *reloadableRegistry) watch(ctx context.Context) {
	t := time.NewTicker(principalsReloadInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			rr.reloadIfChanged()
		}
	}
}

// reloadIfChanged recarga si el mtime del archivo avanzó. Archivo ausente ⇒ no-op (un rm/rename
// transitorio no debe revocar a todos; la revocación real es editar el archivo quitando la línea).
// Un fallo de carga NO avanza lastModNano, así que se reintenta en el próximo tick.
func (rr *reloadableRegistry) reloadIfChanged() {
	fi, err := os.Stat(rr.path)
	if err != nil {
		return
	}
	if fi.ModTime().UnixNano() == rr.lastModNano {
		return
	}
	reg, err := loadPrincipals(rr.path, rr.legacyToken)
	if err != nil {
		logx.Warn("recarga en caliente del registro de principals falló; se conserva el vigente", "path", rr.path, "error", err)
		return
	}
	rr.lastModNano = fi.ModTime().UnixNano()
	rr.cur.Store(reg)
	logx.Info("registro de principals recargado en caliente", "path", rr.path, "principals", len(reg.principals))
}
