package mcp

// Pruebas del CRUCE (fase 5 · S14): el plano de flota preguntándole al de memoria.
//
// Lo que se custodia acá es lo que el dominio no puede: que los términos se compuerten, que la
// memoria se lea en el proyecto de LA MÁQUINA, y que los dos tipos de enlace no se mezclen.

import (
	"strings"
	"testing"
	"time"

	"musubi/internal/embedding"
	"musubi/internal/fleet"
	"musubi/internal/memory"
)

// sembrarContexto deja una máquina con un servicio, actividad, y tres notas distinguibles.
func sembrarContexto(t *testing.T, s *McpServer) fleet.Device {
	t.Helper()
	d := sembrarLosTresPlanos(t, s, "infra", "pc-gio")
	if _, err := s.engine.AltaServicio(fleet.Servicio{
		Nombre: "nginxmarca.service", ProjectID: "infra", DeviceID: d.ID, Clase: "systemd",
	}); err != nil {
		t.Fatal(err)
	}
	// Una nota que NOMBRA el servicio: tiene que enlazar por término.
	if err := s.engine.SaveObservationTypedFrom("infra", "", "obs-termino", "infra/nginx",
		"MARCATERMINO: se cambió el worker_processes de nginxmarca", 1.0, "semantic", "local", nil); err != nil {
		t.Fatal(err)
	}
	// Una nota que NO nombra nada de la máquina: sólo puede enlazar por ventana.
	if err := s.engine.SaveObservationTypedFrom("infra", "", "obs-ventana", "infra/otro",
		"MARCAVENTANA: nota sin relación con esa máquina", 1.0, "semantic", "local", nil); err != nil {
		t.Fatal(err)
	}
	// Un archivo de código tocado en la ventana.
	if err := s.engine.SaveCodeMemoryFrom("infra", memory.CodeMemory{
		Path: "infra/MARCACODIGO.go", Gist: "un gist", Fingerprint: "h", Tokens: 1}); err != nil {
		t.Fatal(err)
	}
	return d
}

func contextoDe(t *testing.T, s *McpServer, p *Principal, args map[string]any) map[string]any {
	t.Helper()
	res, e := callAsPrincipal(t, s, p, "musubi_fleet_contexto", args)
	if e != nil {
		t.Fatalf("contexto: %+v", e)
	}
	return jsonOf(t, res)
}

// LOS DOS ENLACES NO SE MEZCLAN, y el fuerte gana cuando una nota entra por las dos vías.
//
// «Esta nota menciona nginx» y «esta nota se escribió la misma tarde» son evidencias de peso
// incomparable. Presentadas iguales, cualquier coincidencia temporal se lee como una pista.
//
// Sabotaje: rotular todo `ventana` → falla acá.
// Sabotaje: recorrer la ventana ANTES que los términos → la nota que nombra el servicio queda
// rotulada `ventana` y pierde justo el peso que la hace útil.
func TestElEnlacePorTerminoNoSeConfundeConElDeVentana(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	sembrarContexto(t, s)
	p := conCaps("infra", map[fleet.Cap][]string{fleet.CapMetrics: {"*"}, fleet.CapExec: {"*"}})

	out := contextoDe(t, s, p, map[string]any{"device": "pc-gio", "horas": 24})
	enlaces := map[string]string{}
	for _, m := range out["memoria"].([]any) {
		fila := m.(map[string]any)
		if txt, _ := fila["contenido"].(string); strings.Contains(txt, "MARCATERMINO") {
			enlaces["termino"] = fila["enlace"].(string)
			if fila["termino"] == nil {
				t.Error("un enlace por término tiene que decir CUÁL término lo enlazó")
			}
		} else if strings.Contains(txt, "MARCAVENTANA") {
			enlaces["ventana"] = fila["enlace"].(string)
			// El campo `termino` NO va cuando no hubo ninguno: un "" se leería como «no se sabe
			// cuál», que es distinto de «no hubo».
			if _, hay := fila["termino"]; hay {
				t.Error("un enlace por ventana no puede traer un término")
			}
		}
	}
	if enlaces["termino"] != string(fleet.EnlacePorTermino) {
		t.Errorf("la nota que NOMBRA el servicio enlazó como %q, esperaba %q", enlaces["termino"], fleet.EnlacePorTermino)
	}
	if enlaces["ventana"] != string(fleet.EnlacePorVentana) {
		t.Errorf("la nota que sólo coincide en el tiempo enlazó como %q, esperaba %q", enlaces["ventana"], fleet.EnlacePorVentana)
	}
}

// El cruce TRAE las tres cosas: memoria, código y actividad. Es la prueba de humo del cable
// completo, de punta a punta.
//
// LO QUE ESTA PRUEBA NO PRUEBA, y conviene decirlo acá porque su primera versión lo pretendía: no
// custodia el FORMATO de fecha. Con una ventana de 24 h, `2026-08-30 13:50:03` y
// `2026-08-30T12:50:03Z` caen del mismo lado —manda la fecha, no la hora—, así que el sabotaje
// de formatear en RFC3339 la dejaba verde. La guarda de verdad vive en
// TestLaVentanaDeMemoriaUsaElFormatoDeSQLiteYNoRFC3339, con la hora fijada y una ventana del
// mismo día.
func TestElCruceTraeMemoriaCodigoYActividad(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	sembrarContexto(t, s)
	p := conCaps("infra", map[fleet.Cap][]string{fleet.CapMetrics: {"*"}, fleet.CapExec: {"*"}})

	out := contextoDe(t, s, p, map[string]any{"device": "pc-gio", "horas": 24})
	if len(out["memoria"].([]any)) == 0 {
		t.Fatal("la ventana no trajo NINGUNA observación: casi seguro el formato de fecha")
	}
	if len(out["codigo"].([]any)) == 0 {
		t.Fatal("la ventana no trajo NINGÚN archivo de código: casi seguro el formato de fecha")
	}
	crudo := textOf(t, contextoRes(t, s, p, map[string]any{"device": "pc-gio", "horas": 24}))
	if !strings.Contains(crudo, "MARCACODIGO") {
		t.Errorf("el archivo tocado en la ventana no apareció: %s", crudo)
	}
}

func contextoRes(t *testing.T, s *McpServer, p *Principal, args map[string]any) interface{} {
	t.Helper()
	res, e := callAsPrincipal(t, s, p, "musubi_fleet_contexto", args)
	if e != nil {
		t.Fatalf("contexto: %+v", e)
	}
	return res
}

// LOS TÉRMINOS SON INFORMACIÓN Y SE COMPUERTAN. Decirle a alguien «busqué `nginxmarca` en esta
// máquina» le está diciendo que ahí corre un nginx — que es justo lo que `metrics` protege.
//
// Sabotaje: armar los términos de servicio sin preguntar por `metrics` → falla acá, y la tool se
// convierte en un enumerador de servicios que esquiva su propia compuerta.
func TestLosTerminosDeServicioSeCompuertanConMetrics(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	sembrarContexto(t, s)

	sinMetrics := conCaps("infra", map[fleet.Cap][]string{fleet.CapExec: {"*"}})
	out := contextoDe(t, s, sinMetrics, map[string]any{"device": "pc-gio", "horas": 24})

	if out["servicios_ocultos"] != true {
		t.Error("sin `metrics`, la respuesta tiene que DECIR que los servicios quedaron ocultos: si no, se lee como «esta máquina no corre nada»")
	}
	for _, tm := range out["terminos"].([]any) {
		fila := tm.(map[string]any)
		if fila["de"] == string(fleet.TerminoDeServicio) {
			t.Errorf("FUGA: apareció un término de servicio sin `metrics`: %v", fila["texto"])
		}
		if strings.Contains(strings.ToLower(fila["texto"].(string)), "nginxmarca") {
			t.Errorf("FUGA: el nombre del servicio salió igual, por otro campo: %v", fila)
		}
	}
	// El nombre de la MÁQUINA sí sale: quien llegó hasta acá ya la conoce (la nombró en el
	// pedido y el inventario la muestra sin exigir capacidad).
	if len(out["terminos"].([]any)) == 0 {
		t.Error("sin servicios, el término de la máquina tiene que seguir estando")
	}

	// CONTROL: CON `metrics` el término de servicio SÍ aparece. Sin este control, no armar nunca
	// términos de servicio pasaría las aserciones de arriba.
	conMetricsP := conCaps("infra", map[fleet.Cap][]string{fleet.CapMetrics: {"*"}, fleet.CapExec: {"*"}})
	out2 := contextoDe(t, s, conMetricsP, map[string]any{"device": "pc-gio", "horas": 24})
	if out2["servicios_ocultos"] != false {
		t.Error("con `metrics`, servicios_ocultos tiene que ser false")
	}
	hay := false
	for _, tm := range out2["terminos"].([]any) {
		if tm.(map[string]any)["de"] == string(fleet.TerminoDeServicio) {
			hay = true
		}
	}
	if !hay {
		t.Error("con `metrics` tiene que aparecer el término del servicio")
	}
}

// LA MEMORIA SE LEE EN EL PROYECTO DE LA MÁQUINA, no en el alcance de quien pregunta.
//
// Un principal read=all alcanza la memoria de todos los tenants. Si el scope saliera de SU
// alcance, el contexto de una máquina de `infra` traería notas de `otro` — no sería una fuga
// (puede verlas por otra puerta) pero SÍ una respuesta falsa: enlaza como contexto de esa máquina
// algo de otro mundo, y encima con el sello de una herramienta que dice haber correlacionado.
//
// Sabotaje: usar `s.scopedCtx(ctx)` en vez de fijar el proyecto del device → falla acá.
func TestElContextoSaleDeLaMemoriaDeLaMaquinaYNoDeLaDeQuienPregunta(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	sembrarContexto(t, s)
	// Una nota en OTRO proyecto, escrita en la misma ventana.
	if err := s.engine.SaveObservationTypedFrom("otro-tenant", "", "obs-ajena", "otro/tema",
		"MARCAAJENA: esto es de otro proyecto", 1.0, "semantic", "local", nil); err != nil {
		t.Fatal(err)
	}
	// Un admin federado, que SÍ puede ver las dos memorias.
	admin := &Principal{Name: "root", Role: RoleAdmin, Fleet: map[fleet.Cap][]string{
		fleet.CapMetrics: {"*"}, fleet.CapExec: {"*"}}}

	crudo := textOf(t, contextoRes(t, s, admin, map[string]any{"device": "pc-gio", "project": "infra", "horas": 24}))
	if strings.Contains(crudo, "MARCAAJENA") {
		t.Errorf("el contexto de una máquina de `infra` trajo una nota de otro proyecto: %s", crudo)
	}
	// CONTROL POSITIVO: la nota de SU proyecto sí está. Sin esto, un scope roto que no devuelva
	// nada pasaría la aserción de arriba.
	if !strings.Contains(crudo, "MARCAVENTANA") && !strings.Contains(crudo, "MARCATERMINO") {
		t.Fatalf("no vino ninguna nota del propio proyecto: el scope quedó demasiado angosto: %s", crudo)
	}
}

// LA ACTIVIDAD SE COMPUERTA HECHO POR HECHO, con la MISMA función que la cronología.
//
// Sabotaje: contar todos los hechos sin pasar por hechosVisiblesPara → falla acá, y el resumen
// le diría a alguien con sólo `exec` cuántas veces entraron a la pantalla de esa máquina.
func TestLaActividadDelContextoSeCompuertaComoLaCronologia(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	sembrarContexto(t, s)

	soloExec := conCaps("infra", map[fleet.Cap][]string{fleet.CapExec: {"*"}})
	out := contextoDe(t, s, soloExec, map[string]any{"device": "pc-gio", "horas": 24})
	act := out["actividad"].(map[string]any)
	tipos := act["por_tipo"].(map[string]any)

	if tipos["comando"] == nil {
		t.Error("con `exec` tiene que ver sus comandos")
	}
	if tipos["sesion_pantalla"] != nil || tipos["sesion_shell"] != nil {
		t.Errorf("FUGA: con sólo `exec` se contaron sesiones de otros planos: %v", tipos)
	}
	if n, _ := act["ocultos_por_permiso"].(float64); n < 2 {
		t.Errorf("ocultos_por_permiso = %v: tiene que decir que hay más y que falta permiso", act["ocultos_por_permiso"])
	}
}

// LOS HUECOS VIAJAN SIEMPRE, también en una respuesta llena. Una lista de coincidencias sin sus
// límites al lado se lee como una explicación.
//
// Sabotaje: devolver nil desde HuecosDelContexto → falla acá.
func TestElContextoDeclaraQueEsCorrelacionYNoCausa(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	sembrarContexto(t, s)
	p := conCaps("infra", map[fleet.Cap][]string{fleet.CapMetrics: {"*"}, fleet.CapExec: {"*"}})

	out := contextoDe(t, s, p, map[string]any{"device": "pc-gio", "horas": 24})
	huecos, _ := out["no_visto"].([]any)
	if len(huecos) == 0 {
		t.Fatal("la respuesta no declaró ningún límite")
	}
	junto := strings.ToLower(strings.Join(func() []string {
		var xs []string
		for _, h := range huecos {
			xs = append(xs, h.(string))
		}
		return xs
	}(), " | "))
	for _, obligatorio := range []string{"correlación", "causa", "homónimo", "commit"} {
		if !strings.Contains(junto, obligatorio) {
			t.Errorf("falta declarar %q en los huecos: %s", obligatorio, junto)
		}
	}
}

// Una máquina de otro tenant no tiene contexto, y el error no confirma que exista.
func TestElContextoNoEsUnOraculoDeMaquinasAjenas(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	sembrarContexto(t, s)
	ajeno := conCaps("otro", map[fleet.Cap][]string{fleet.CapMetrics: {"*"}, fleet.CapExec: {"*"}})
	_, e := callAsPrincipal(t, s, ajeno, "musubi_fleet_contexto",
		map[string]any{"device": "pc-gio", "project": "infra"})
	if e == nil {
		t.Fatal("una credencial de otro proyecto obtuvo el contexto de una máquina ajena")
	}
	if strings.Contains(e.Message, "MARCATERMINO") || strings.Contains(e.Message, "nginxmarca") {
		t.Errorf("el mensaje filtró dato ajeno: %s", e.Message)
	}
}

// El contenido largo se recorta CON MARCA. Una nota cortada sin marca se lee como una nota corta,
// y alguien concluye sobre media frase creyendo que la leyó entera.
func TestUnaNotaLargaSeRecortaConMarca(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	sembrarContexto(t, s)
	larga := "MARCALARGA " + strings.Repeat("á", fleet.ContenidoMax+200)
	if err := s.engine.SaveObservationTypedFrom("infra", "", "obs-larga", "infra/larga",
		larga, 1.0, "semantic", "local", nil); err != nil {
		t.Fatal(err)
	}
	p := conCaps("infra", map[fleet.Cap][]string{fleet.CapMetrics: {"*"}, fleet.CapExec: {"*"}})
	out := contextoDe(t, s, p, map[string]any{"device": "pc-gio", "horas": 24})
	for _, m := range out["memoria"].([]any) {
		txt := m.(map[string]any)["contenido"].(string)
		if strings.HasPrefix(txt, "MARCALARGA") {
			if !strings.Contains(txt, "[recortado]") {
				t.Error("una nota recortada tiene que decirlo")
			}
			// Se corta por RUNAS: con acentos, cortar por bytes parte un carácter al medio.
			if strings.ContainsRune(txt, '�') {
				t.Error("el recorte partió un carácter multibyte")
			}
			return
		}
	}
	t.Fatal("la nota larga no apareció en el contexto")
}

// Una ventana donde no pasó nada ni se escribió nada devuelve listas vacías, PERO con los huecos
// y con `servicios_ocultos` bien puesto: el vacío tiene que poder distinguirse de la ceguera.
func TestUnaVentanaVaciaSigueDiciendoQueNoMiro(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	sembrarContexto(t, s)
	p := conCaps("infra", map[fleet.Cap][]string{fleet.CapMetrics: {"*"}, fleet.CapExec: {"*"}})

	// Una ventana de hace tres días: no hay actividad ni notas ahí.
	viejo := time.Now().UTC().Add(-72 * time.Hour)
	out := contextoDe(t, s, p, map[string]any{
		"device": "pc-gio",
		"desde":  viejo.Format(time.RFC3339),
		"hasta":  viejo.Add(time.Hour).Format(time.RFC3339),
	})
	act := out["actividad"].(map[string]any)
	if n, _ := act["total"].(float64); n != 0 {
		t.Errorf("actividad en una ventana vieja: %v", act)
	}
	if len(out["no_visto"].([]any)) == 0 {
		t.Error("una respuesta vacía es JUSTO la que más necesita declarar qué no miró")
	}
	// Los términos SIGUEN estando: no dependen de la ventana, y son lo que explica por qué la
	// búsqueda por término no encontró nada.
	if len(out["terminos"].([]any)) == 0 {
		t.Error("los términos no dependen de la ventana y tienen que seguir declarándose")
	}
}

// EL ENLACE POR TÉRMINO ES UNA FRASE, NO EL OR DE SUS TOKENS.
//
// Un servicio llamado `cognicion-db` buscado con OR enlaza cualquier nota que diga «db» — y la
// respuesta afirmaría que ese texto NOMBRA algo de la máquina cuando no lo nombra. Un `ventana`
// mal puesto agrega ruido; un `termino` mal puesto INVENTA EVIDENCIA, que es el único error que
// esta tool no se puede permitir.
//
// Lo encontré usando la tool contra la flota real: la primera corrida devolvió una nota sobre
// decisiones de roadmap enlazada a `avahi-daemon`.
//
// Sabotaje: volver a `SearchObservationsFTS` (que une los tokens con OR) → falla acá.
func TestElEnlacePorTerminoBuscaLaFraseYNoSusPedazos(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	d := sembrarLosTresPlanos(t, s, "infra", "pc-gio")
	if _, err := s.engine.AltaServicio(fleet.Servicio{
		Nombre: "cognicionmarca-db", ProjectID: "infra", DeviceID: d.ID, Clase: "systemd",
	}); err != nil {
		t.Fatal(err)
	}
	// Una nota que dice SÓLO uno de los pedazos: con OR entraría como `termino`.
	if err := s.engine.SaveObservationTypedFrom("infra", "", "obs-pedazo", "infra/otro",
		"MARCAPEDAZO: hablamos de la db de otro sistema", 1.0, "semantic", "local", nil); err != nil {
		t.Fatal(err)
	}
	// Y una que SÍ lo nombra entero.
	if err := s.engine.SaveObservationTypedFrom("infra", "", "obs-entero", "infra/ese",
		"MARCAENTERO: se reinició cognicionmarca-db anoche", 1.0, "semantic", "local", nil); err != nil {
		t.Fatal(err)
	}
	p := conCaps("infra", map[fleet.Cap][]string{fleet.CapMetrics: {"*"}, fleet.CapExec: {"*"}})
	out := contextoDe(t, s, p, map[string]any{"device": "pc-gio", "horas": 24})

	for _, m := range out["memoria"].([]any) {
		fila := m.(map[string]any)
		txt, _ := fila["contenido"].(string)
		if strings.Contains(txt, "MARCAPEDAZO") && fila["enlace"] == string(fleet.EnlacePorTermino) {
			t.Errorf("EVIDENCIA INVENTADA: una nota que sólo dice «db» quedó enlazada por término a `cognicionmarca-db`: %v", fila)
		}
		if strings.Contains(txt, "MARCAENTERO") && fila["enlace"] != string(fleet.EnlacePorTermino) {
			t.Errorf("la nota que NOMBRA el servicio entero no enlazó por término: %v", fila)
		}
	}
}

// LA TOOL LEE `Declarado` DE VERDAD, no sólo el dominio sabe ordenarlos.
//
// Un host enumera decenas de units y el tope de términos se llena con las primeras. El servicio
// que una PERSONA declaró a mano —el bot, el puente— es el único del que suele haber algo escrito,
// y es justo el que se perdía. Medido contra la flota real: `alturito20` quedó afuera mientras
// entraban `avahi-daemon` y `NetworkManager-wait-online`.
//
// Sabotaje: mandar todos los servicios a `reportados` sin mirar `sv.Declarado` → falla acá.
func TestElServicioDeclaradoLlegaALosTerminosAunqueElHostEnumereMuchos(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	d := sembrarLosTresPlanos(t, s, "infra", "pc-gio")

	// Lo que enumera la máquina: más units que ranuras.
	var reportes []fleet.ReporteServicio
	for i := 0; i < fleet.TerminosMax+5; i++ {
		reportes = append(reportes, fleet.ReporteServicio{
			Nombre: "aaaunitdelsistema" + string(rune('a'+i)), Clase: "systemd", Salud: fleet.SaludServicio{Tomada: time.Now().UTC(), Estado: fleet.EstadoCorriendo},
		})
	}
	if _, _, err := s.engine.ReportarServicios(d.ID, time.Now().UTC(), reportes); err != nil {
		t.Fatal(err)
	}
	// Y lo que declaró una persona. EL NOMBRE ARRANCA CON `zzz` A PROPÓSITO: con un nombre que
	// ordena primero, gana la ranura por orden alfabético y la prueba pasa aunque nadie mire
	// `Declarado`. Lo descubrí ejecutando el sabotaje, que quedaba verde.
	if _, err := s.engine.AltaServicio(fleet.Servicio{
		Nombre: "zzzalturitomarca", ProjectID: "infra", DeviceID: d.ID,
	}); err != nil {
		t.Fatal(err)
	}

	p := conCaps("infra", map[fleet.Cap][]string{fleet.CapMetrics: {"*"}, fleet.CapExec: {"*"}})
	out := contextoDe(t, s, p, map[string]any{"device": "pc-gio", "horas": 24})

	hay := false
	for _, tm := range out["terminos"].([]any) {
		if tm.(map[string]any)["texto"] == "zzzalturitomarca" {
			hay = true
		}
	}
	if !hay {
		t.Fatalf("el servicio DECLARADO por una persona no llegó a los términos: %v", out["terminos"])
	}
}
