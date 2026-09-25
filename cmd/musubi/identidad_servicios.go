package main

// identidad_servicios.go es CON QUIÉN SE ENUMERA el mundo del dueño cuando el agente corre como root.
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// POR QUÉ HACE FALTA, MEDIDO Y NO SUPUESTO
//
// El dueño decidió el 2026-09-24 que el agente de `musubi-server` corra como root: todo `exec` y
// toda `shell` concedidos sobre esa máquina, como root. Pero lo que el agente ENUMERA no es de
// root. Los 18 contenedores son podman ROOTLESS del usuario `musubi` (uid 1000), y las units que
// escribió el dueño (los puentes de WhatsApp, el gateway, el CRM, Core01) son units `--user` de
// ese mismo usuario. Con el binario de antes, un agente root:
//
//   - corría `podman ps` como root, que lee el store ROOTFUL (/var/lib/containers): vacío. El
//     cerebro poda por ausencia, así que los 18 contenedores se daban de BAJA en el primer latido;
//   - y un `systemctl --user` como root mira el manager de root, no el de musubi, y se quedaba
//     «ausente» en silencio.
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// EL MECANISMO: LA CREDENCIAL DE GO, Y SÓLO PARA ENUMERAR
//
// `SysProcAttr.Credential` baja el hijo a otro uid/gid sin binario externo ni PAM (`runuser` deja
// una sesión en /var/log/secure por cada latido). Se usa ÚNICAMENTE para las dos fuentes que son
// del dueño —podman y `systemctl --user`—. El exec y la shell de las personas, y el `systemctl` del
// sistema, siguen corriendo como root: ésa es la decisión del dueño, y esto no la toca.
//
// La identidad sale de `MUSUBI_AGENTE_USUARIO`, que el drop-in de producción pone en `musubi`. Sin
// la variable, un agente root enumera el mundo de root —que es lo coherente— y lo dice UNA vez al
// arrancar, para que nadie crea que está mirando lo del dueño.

import (
	"fmt"
	"os/user"
	"strconv"
	"strings"
)

// envUsuarioDeServicios nombra al usuario cuyo mundo enumera un agente que corre como root.
const envUsuarioDeServicios = "MUSUBI_AGENTE_USUARIO"

// identidadDeServicios es el usuario con el que se enumeran podman y las units `--user`.
//
// Bajar es la única decisión que importa: con false el hijo hereda al agente tal cual (su uid y su
// entorno), que es lo que pasaba siempre; con true el hijo baja a Uid/Gid/Grupos y lleva el
// entorno del dueño. El valor cero es «no bajar», así que un agente que nunca resolvió la identidad
// —y las pruebas que no la tocan— se comportan exactamente como antes.
type identidadDeServicios struct {
	Nombre   string
	Uid, Gid uint32
	Grupos   []uint32
	// Home es donde viven las units que el dueño escribió (~/.config/systemd/user) y su store de
	// podman. Runtime es su /run/user/<uid>: el bus de su manager y los locks de podman.
	Home, Runtime string
	Bajar         bool
}

// cuentaDelSistema es lo que se necesita saber de un usuario del passwd. Es un tipo propio y no
// *user.User para que las pruebas puedan fabricar cuentas sin que existan en la máquina.
type cuentaDelSistema struct {
	Nombre   string
	Uid, Gid uint32
	Grupos   []uint32
	Home     string
}

// resolverIdentidadDeServicios decide con quién se enumera. Los seis casos, y por qué:
//
//	uid>0, sin variable            → la propia: el agente ya ES el dueño (el despliegue de siempre)
//	uid>0, variable = él mismo      → la propia: redundante pero coherente
//	uid>0, variable = OTRO          → error: un proceso sin privilegios no puede bajar a nadie, y
//	                                  seguir como si nada enumeraría el mundo equivocado en silencio
//	uid 0, sin variable            → el mundo de root, dicho en voz alta al arrancar
//	uid 0, variable = inexistente   → error: arrancar enumerando otra cosa es la baja de los 18
//	uid 0, variable = root          → error: «bajar» a root no baja nada y esconde un typo
//	uid 0, variable = otro          → BAJA a ese usuario, con su home y su /run/user/<uid>
//
// uid<0 es un sistema sin uid (Windows): ahí la variable no significa nada y se rechaza, por la
// misma razón que un uid>0 con otro nombre.
//
// El runtime de la identidad PROPIA es el XDG_RUNTIME_DIR que heredan los hijos —el que runAgent
// exportó si hacía falta—, no uno deducido: si el hijo no lo tiene, `systemctl --user` no llega al
// bus, y anunciar un bus que el hijo no va a ver sería abortar el inventario por nada.
func resolverIdentidadDeServicios(uid int, getenv func(string) string, buscar func(string) (cuentaDelSistema, error)) (identidadDeServicios, error) {
	pedido := strings.TrimSpace(getenv(envUsuarioDeServicios))
	propia := identidadDeServicios{
		Nombre:  strings.TrimSpace(getenv("USER")),
		Home:    strings.TrimSpace(getenv("HOME")),
		Runtime: strings.TrimSpace(getenv("XDG_RUNTIME_DIR")),
	}
	if uid >= 0 {
		propia.Uid = uint32(uid) // el gid propio no hace falta: sin bajar no se le pide nada al sistema
	}
	if propia.Nombre == "" && uid == 0 {
		propia.Nombre = "root"
	}

	if uid != 0 {
		if pedido == "" {
			return propia, nil
		}
		if uid < 0 {
			return identidadDeServicios{}, fmt.Errorf("%s=%q: en este sistema no hay uid al que bajar; "+
				"la variable sólo tiene sentido en un agente Unix que corre como root", envUsuarioDeServicios, pedido)
		}
		c, err := buscar(pedido)
		if err != nil || int64(c.Uid) != int64(uid) {
			return identidadDeServicios{}, fmt.Errorf("%s=%q y este agente corre con uid %d: sin ser root "+
				"no puede enumerar el mundo de otro usuario. Sacá la variable o corré el agente como root",
				envUsuarioDeServicios, pedido, uid)
		}
		return propia, nil
	}

	if pedido == "" {
		return propia, nil
	}
	c, err := buscar(pedido)
	if err != nil {
		return identidadDeServicios{}, fmt.Errorf("%s=%q: ese usuario no existe en esta máquina (%v). "+
			"Arrancar igual enumeraría el mundo de root y el cerebro daría de BAJA todo lo del dueño",
			envUsuarioDeServicios, pedido, err)
	}
	if c.Uid == 0 {
		return identidadDeServicios{}, fmt.Errorf("%s=%q es root: bajar a root no baja nada. "+
			"Nombrá al dueño de los contenedores y de las units --user (en musubi-server, `musubi`)",
			envUsuarioDeServicios, pedido)
	}
	return identidadDeServicios{
		Nombre:  c.Nombre,
		Uid:     c.Uid,
		Gid:     c.Gid,
		Grupos:  c.Grupos,
		Home:    c.Home,
		Runtime: "/run/user/" + strconv.FormatUint(uint64(c.Uid), 10),
		Bajar:   true,
	}, nil
}

// describir es la línea de arranque. Vacía en un sistema sin uid: ahí no hay nada que decidir.
func (id identidadDeServicios) describir(uid int) string {
	switch {
	case uid < 0:
		return ""
	case id.Bajar:
		return fmt.Sprintf("inventario de usuario: %s (uid %d)", id.Nombre, id.Uid)
	case uid == 0:
		return fmt.Sprintf("inventario de usuario: root (uid 0) · sin %s, podman y las units --user "+
			"que se enumeran son las de ROOT, no las del dueño", envUsuarioDeServicios)
	default:
		return fmt.Sprintf("inventario de usuario: %s (uid %d)", id.Nombre, id.Uid)
	}
}

// buscarCuenta lee el passwd y el group de la máquina. Con CGO_ENABLED=0 —que es como se compila el
// binario— os/user los lee directo de /etc/passwd y /etc/group, sin NSS.
//
// SI LOS GRUPOS SUPLEMENTARIOS NO SE PUEDEN LEER, EL HIJO BAJA SÓLO CON SU GRUPO PRIMARIO: es menos
// privilegio, no más, y para `podman ps` o `systemctl --user show` no hace falta ningún otro.
func buscarCuenta(nombre string) (cuentaDelSistema, error) {
	u, err := user.Lookup(nombre)
	if err != nil {
		return cuentaDelSistema{}, err
	}
	uid, err := strconv.ParseUint(u.Uid, 10, 32)
	if err != nil {
		return cuentaDelSistema{}, fmt.Errorf("uid %q de %s no es numérico: %w", u.Uid, nombre, err)
	}
	gid, err := strconv.ParseUint(u.Gid, 10, 32)
	if err != nil {
		return cuentaDelSistema{}, fmt.Errorf("gid %q de %s no es numérico: %w", u.Gid, nombre, err)
	}
	c := cuentaDelSistema{Nombre: u.Username, Uid: uint32(uid), Gid: uint32(gid), Home: u.HomeDir}
	if ids, err := u.GroupIds(); err == nil {
		for _, s := range ids {
			if g, err := strconv.ParseUint(s, 10, 32); err == nil {
				c.Grupos = append(c.Grupos, uint32(g))
			}
		}
	}
	if len(c.Grupos) == 0 {
		c.Grupos = []uint32{c.Gid}
	}
	return c, nil
}

// entornoPara arma el entorno del hijo que BAJA. Parte del del agente y le saca lo que es de root.
//
// NO ES COSMÉTICO: podman rootless decide su store con HOME y XDG_CONFIG_HOME, y sus locks con
// XDG_RUNTIME_DIR. Un hijo con uid 1000 y HOME=/root no puede leer /root, y uno con
// XDG_CONFIG_HOME=/root/.config buscaría la configuración de root. `systemctl --user` sin
// XDG_RUNTIME_DIR no encuentra el bus. Y las credenciales del agente —el token, en archivo o en la
// variable— no tienen por qué viajar a un proceso de otro usuario.
//
// XDG_RUNTIME_DIR SE PONE SÓLO SI EL DIRECTORIO EXISTE Y ES DEL UID: apuntar a uno que no existe
// hace fallar a podman rootless, que abortaría el inventario entero; y uno ajeno no es su bus.
func entornoPara(id identidadDeServicios, base []string, esDe func(dir string, uid uint32) bool) []string {
	fuera := func(clave string) bool {
		switch clave {
		case "HOME", "USER", "LOGNAME", "DBUS_SESSION_BUS_ADDRESS", envTokenFile, envToken:
			return true
		}
		return strings.HasPrefix(clave, "XDG_")
	}
	out := make([]string, 0, len(base)+4)
	for _, kv := range base {
		clave, _, _ := strings.Cut(kv, "=")
		if fuera(clave) {
			continue
		}
		out = append(out, kv)
	}
	out = append(out, "HOME="+id.Home)
	out = append(out, "USER="+id.Nombre, "LOGNAME="+id.Nombre)
	if id.Runtime == "" || !esDe(id.Runtime, id.Uid) {
		return out
	}
	return append(out, "XDG_RUNTIME_DIR="+id.Runtime)
}

// prepararIdentidadDeServicios es lo que runAgent hace antes del primer latido: exportar el runtime
// propio si hace falta (runtimeParaHeredar) y, RECIÉN DESPUÉS, resolver con quién se enumera.
//
// EL ORDEN ES LA MITAD DEL CONTRATO. La identidad propia toma su Runtime del XDG_RUNTIME_DIR del
// entorno —el que van a heredar los hijos—, y systemd no lo exporta en una unidad de sistema con
// `User=`. Resolver primero dejaba al agente `musubi` de musubi-server con Runtime vacío: la fuente
// `--user` se leía como «ausente», en silencio, y ninguna `usuario:*` llegaba al inventario.
func prepararIdentidadDeServicios(uid int, getenv func(string) string, setenv func(string, string) error,
	buscar func(string) (cuentaDelSistema, error), esDe func(dir string, uid uint32) bool) (identidadDeServicios, error) {
	if d, ok := runtimeParaHeredar(getenv, uid, func(d string) bool { return esDe(d, uint32(uid)) }); ok {
		_ = setenv("XDG_RUNTIME_DIR", d)
	}
	return resolverIdentidadDeServicios(uid, getenv, buscar)
}

// identidadParaEnumerar es la que runAgent resolvió al arrancar. El valor cero es «no bajar».
var identidadParaEnumerar identidadDeServicios
