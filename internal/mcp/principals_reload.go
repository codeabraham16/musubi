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
// ImpactoDeNombre dice QUÉ AUTORIZACIONES NOMBRAN a una máquina, y existe porque renombrar un
// device NO es cosmético: es un cambio de autorización disfrazado (A64).
//
// Tres cosas de este repo indexan por NOMBRE de máquina y ninguna por id:
//
//	tieneGrant        → las concesiones de capacidad de `principals.yaml`
//	argvPermitido     → la allowlist de comandos por máquina (`exec_allow`)
//	Politica.Alcanza  → a qué máquinas alcanza una política (`config.yaml`)
//
// Así que un rename le puede SACAR `exec` a alguien, o DÁRSELO, o meter una máquina adentro del
// alcance de una política — sin que nadie lo haya pedido y sin que quede rastro de por qué. Este
// tipo es lo que permite decirlo ANTES, en vez de que se descubra cuando algo deja de andar.
type ImpactoDeNombre struct {
	// Concesiones son los principals cuya sección `fleet:` nombra esta máquina.
	Concesiones []string
	// Allowlists son los principals con una entrada de `exec_allow` para esta máquina. Van
	// aparte de las concesiones porque se pierden distinto: quedarse sin concesión niega el
	// acceso —ruidoso, se nota—; quedarse sin entrada de allowlist con la SECCIÓN presente
	// deniega TODO comando por el paso 4 de argvPermitido, que es igual de silencioso y mucho
	// más confuso.
	Allowlists []string
}

func (i ImpactoDeNombre) Vacio() bool { return len(i.Concesiones) == 0 && len(i.Allowlists) == 0 }

type principalResolver interface {
	resolve(token string) (*Principal, bool)
	// impactoDeNombre lista qué credenciales NOMBRAN esta máquina. Va en esta interfaz y no en
	// una aparte por el mismo motivo que porNombre: es la misma fuente de verdad, y dos
	// interfaces separadas invitarían a que el informe de impacto mire un registro más viejo que
	// el que autentica.
	impactoDeNombre(device string) ImpactoDeNombre
	// porNombre resuelve SIN token: lo necesitan las políticas de flota (S10), que actúan con la
	// autoridad de alguien declarado en principals.yaml pero no presentan credencial ninguna.
	// Está en la misma interfaz que resolve a propósito — son la misma fuente de verdad, y dos
	// interfaces separadas invitarían a que una política mire un registro más viejo que el que
	// autentica a las personas.
	porNombre(nombre string) (*Principal, bool)
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

// porNombre busca en el snapshot vigente (lock-free), igual que resolve. Que las dos preguntas
// salgan del MISMO snapshot es lo que hace que revocar a alguien en principals.yaml apague, en el
// mismo instante, tanto su token como las políticas que actuaban en su nombre.
// impactoDeNombre delega en el snapshot VIGENTE, igual que las otras dos. Que las tres lean el
// mismo puntero es lo que impide que el informe de impacto de un rename describa un registro que
// ya no es el que autoriza.
func (rr *reloadableRegistry) impactoDeNombre(device string) ImpactoDeNombre {
	if reg := rr.cur.Load(); reg != nil {
		return reg.impactoDeNombre(device)
	}
	return ImpactoDeNombre{}
}

func (rr *reloadableRegistry) porNombre(nombre string) (*Principal, bool) {
	reg := rr.cur.Load()
	if reg == nil {
		return nil, false
	}
	return reg.porNombre(nombre)
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
