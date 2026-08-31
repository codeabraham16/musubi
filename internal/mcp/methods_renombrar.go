package mcp

// methods_renombrar.go es `musubi_fleet_rename` (A64): cambiarle el nombre a una máquina sin
// perder su historial — y sin cambiarle la autorización por accidente.
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// RENOMBRAR NO ES COSMÉTICO: ES UN CAMBIO DE AUTORIZACIÓN DISFRAZADO
//
// Tres cosas de este repo indexan por NOMBRE de máquina y ninguna por id:
//
//	tieneGrant        → las concesiones de capacidad de `principals.yaml`
//	argvPermitido     → la allowlist de comandos por máquina (`exec_allow`)
//	Politica.Alcanza  → a qué máquinas alcanza una política (`config.yaml`)
//
// Así que renombrar puede SACARLE `exec` a alguien, DÁRSELO, o meter una máquina adentro del
// alcance de una política que la va a tocar sola — todo en silencio, y con el síntoma apareciendo
// días después como «esto dejó de andar».
//
// Por eso la tool NO renombra en el primer llamado. Informa el impacto y se planta. Es la misma
// forma que tiene todo lo caro de este track: el default de «no lo pensé» es «no pasa nada».
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// POR QUÉ NO ARREGLA `principals.yaml` SOLO
//
// Sería un cerebro editando la credencial de una persona. La regla del track es la contraria:
// las concesiones se escriben a mano, y ni siquiera hay tool para otorgarlas (B3). Lo que sí se
// puede hacer —y es lo que falta hoy— es DECIR exactamente qué quedó apuntando a un nombre que ya
// no existe, para que el arreglo sea de dos minutos en vez de una tarde de desconcierto.

import (
	"context"
	"encoding/json"
	"strings"

	"musubi/internal/fleet"
	"musubi/internal/logx"
)

func (s *McpServer) toolFleetRename(ctx context.Context, raw json.RawMessage) (interface{}, *RpcError) {
	p := principalFrom(ctx)
	if !p.isAdmin() {
		return nil, rpcErrorf(codeUnauthorized,
			"renombrar una máquina es admin: cambia a qué apuntan las concesiones de principals.yaml")
	}
	var args struct {
		Device    string `json:"device"`
		Nuevo     string `json:"nuevo"`
		Project   string `json:"project"`
		Confirmar bool   `json:"confirmar"`
	}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &args); err != nil {
			return nil, rpcErrorf(codeInvalidParams, "argumentos inválidos: %v", err)
		}
	}
	viejo, nuevo := strings.TrimSpace(args.Device), strings.TrimSpace(args.Nuevo)
	if viejo == "" || nuevo == "" {
		return nil, rpcErrorf(codeInvalidParams, "faltan `device` (el nombre actual) y `nuevo`")
	}
	if !fleet.NombreDeDeviceValido(nuevo) {
		return nil, rpcErrorf(codeInvalidParams,
			"%q no sirve como nombre de máquina: 1..%d runas, sin caracteres de control y sin comas (la columna de tags es CSV)",
			nuevo, fleet.NombreDeviceMax)
	}
	device, proyecto, rpcErr := s.resolverDeviceUnico(p, viejo, args.Project)
	if rpcErr != nil {
		return nil, rpcErr
	}

	// ── El impacto, ANTES de tocar nada ──────────────────────────────────────────────────────
	//
	// Se mira el nombre VIEJO —lo que va a dejar de existir— y también el NUEVO: si algo ya
	// nombraba el nombre nuevo, la máquina renombrada HEREDA esa autorización sin que nadie se
	// la haya dado. Ése es el sentido peligroso, y el que a nadie se le ocurre mirar.
	pierde := s.impactoDe(viejo)
	hereda := s.impactoDe(nuevo)
	politicasViejo := s.politicasQueNombran(viejo)
	politicasNuevo := s.politicasQueNombran(nuevo)

	informe := map[string]interface{}{
		"device":  device.Name,
		"nuevo":   nuevo,
		"project": proyecto,
		"pierden_de_vista": map[string]interface{}{
			"concesiones": pierde.Concesiones,
			"allowlists":  pierde.Allowlists,
			"politicas":   politicasViejo,
		},
		"hereda_del_nombre_nuevo": map[string]interface{}{
			"concesiones": hereda.Concesiones,
			"allowlists":  hereda.Allowlists,
			"politicas":   politicasNuevo,
		},
		// LO QUE NO CAMBIA SE DICE TAMBIÉN. La mitad del miedo a renombrar es no saber qué se
		// lleva puesto, y enumerar sólo los riesgos deja creer que se pierde todo.
		"conserva": []string{
			"el id de la máquina, y con él su bitácora de comandos entera",
			"sus sesiones de pantalla y de shell",
			"su inventario de servicios y la salud de cada uno",
			"su token: el agente sigue latiendo sin reinstalar nada",
		},
		// LO QUE ESTE INFORME NO PUEDE VER SE DICE, EN VEZ DE CALLARLO (A66).
		//
		// El informe enumera lo que nombra a esta máquina DENTRO del cerebro —`principals.yaml` y
		// `config.yaml`— y con eso da la impresión de ser exhaustivo. No lo es: `device` es también
		// una ETIQUETA de Prometheus, y ahí el nombre viejo no se migra ni se borra. Sus series
		// dejan de actualizarse y envejecen con la retención; las nuevas arrancan sin historia. Una
		// consulta, un panel o una alerta que filtre por el nombre viejo no falla — deja de
		// disparar, que es peor.
		//
		// El cerebro no puede COMPROBARLO: le empuja métricas a Prometheus, no le consulta. Así que
		// se declara el hecho en vez de inventarle una verificación que no puede hacer. Un informe
		// que calla lo que no mira se lee como si lo hubiera mirado.
		"no_puedo_ver": []string{
			"Prometheus: `device` es una etiqueta, así que las series del nombre viejo quedan " +
				"huérfanas (envejecen con la retención) y las del nuevo arrancan sin historia. " +
				"Revisá consultas, paneles y alertas que filtren por el nombre viejo: no fallan, " +
				"dejan de disparar.",
		},
	}

	if !args.Confirmar {
		informe["renombrado"] = false
		informe["que_hacer"] = queHacerAntesDeRenombrar(pierde, hereda, politicasViejo, politicasNuevo)
		return jsonResult(informe)
	}

	renombrado, err := s.engine.RenombrarDevice(proyecto, viejo, nuevo)
	if err != nil {
		return nil, rpcErrorf(codeInvalidParams, "%v", err)
	}
	// QUEDA EN EL LOG DEL CEREBRO con las dos puntas. Un rename es de las pocas operaciones que
	// no deja rastro en ninguna tabla —cambia una columna y listo—, así que sin esta línea la
	// pregunta «¿por qué esta máquina se llama distinto?» no tiene respuesta en ningún lado.
	logx.Info("flota: máquina renombrada", "proyecto", proyecto, "de", viejo, "a", nuevo,
		"device_id", renombrado.ID, "principal", nombrePrincipal(p),
		"concesiones_que_la_nombraban", len(pierde.Concesiones),
		"politicas_que_la_nombraban", len(politicasViejo))
	informe["renombrado"] = true
	informe["device"] = renombrado.Name
	return jsonResult(informe)
}

// impactoDe consulta el registro vigente. Sin registro —stdio local, bearer legacy— no hay
// concesiones por nombre que romper, y devolver vacío es la verdad y no una omisión.
func (s *McpServer) impactoDe(nombre string) ImpactoDeNombre {
	if s.buscarPrincipal == nil {
		return ImpactoDeNombre{}
	}
	return s.buscarPrincipal.impactoDeNombre(nombre)
}

// politicasQueNombran lista las políticas cuyo alcance nombra esta máquina. El comodín no cuenta,
// por el mismo motivo que en las concesiones: una política sobre todas sobrevive al rename, y
// listarla sería ruido que tapa lo que sí se rompe.
func (s *McpServer) politicasQueNombran(nombre string) []string {
	var out []string
	for _, pol := range s.politicas {
		// `Sobre` y no un helper: `Alcanza` también acepta el comodín, y acá interesa lo que
		// NOMBRA a esta máquina — una política sobre todas sobrevive al rename.
		for _, d := range fleet.LimpiarSelectores(pol.Sobre) {
			if d == nombre {
				out = append(out, pol.Nombre)
				break
			}
		}
	}
	return out
}

// queHacerAntesDeRenombrar convierte el impacto en instrucciones. Un informe que enumera riesgos
// y no dice qué hacer con ellos se lee, se asiente y no se actúa.
func queHacerAntesDeRenombrar(pierde, hereda ImpactoDeNombre, polViejo, polNuevo []string) []string {
	var pasos []string
	if len(pierde.Concesiones) > 0 {
		pasos = append(pasos, "En `principals.yaml`, cambiá el nombre viejo por el nuevo en la sección `fleet:` de: "+
			strings.Join(pierde.Concesiones, ", ")+". Si no, esas credenciales dejan de alcanzar esta máquina.")
	}
	if len(pierde.Allowlists) > 0 {
		pasos = append(pasos, "En `principals.yaml`, renombrá la entrada de `exec_allow` de: "+
			strings.Join(pierde.Allowlists, ", ")+
			". OJO: con la sección presente y sin entrada para esta máquina, NO se le puede correr NINGÚN comando — se deniega en silencio.")
	}
	if len(polViejo) > 0 {
		pasos = append(pasos, "En `config.yaml`, actualizá el `devices:` de las políticas: "+
			strings.Join(polViejo, ", ")+". Si no, dejan de actuar sobre esta máquina.")
	}
	if len(hereda.Concesiones) > 0 || len(hereda.Allowlists) > 0 || len(polNuevo) > 0 {
		pasos = append(pasos, "⚠ EL NOMBRE NUEVO YA ESTÁ NOMBRADO por otras credenciales o políticas: al renombrar, "+
			"esta máquina HEREDA esa autorización sin que nadie se la haya dado. Revisá que sea lo que querés antes de confirmar.")
	}
	if len(pasos) == 0 {
		pasos = append(pasos, "Nada nombra a esta máquina por su nombre: el rename no cambia ninguna autorización. "+
			"Volvé a llamar con `confirmar: true`.")
	} else {
		pasos = append(pasos, "Hecho eso, volvé a llamar con `confirmar: true`. El registro recarga solo en ≤10 s.")
	}
	return pasos
}
