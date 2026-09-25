package mcp

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"musubi/internal/embedding"
)

// Las series de vencimiento, en el orden en que las emite principals_metricas.go.
var seriesDeVencimiento = []string{
	nombreProximoVencimiento,
	nombreCredencialesVencidas,
	nombreCredencialesEternas,
	nombreLegacyHabilitado,
	nombreLegacyAciertos,
	nombreRegistroSinRecargar,
	nombreRegistroRecargasMal,
}

// registroRecargableDesde carga el YAML como lo carga el cerebro al arrancar y lo envuelve en el
// registro recargable, que es el que corre en producción siempre que hay archivo. Entrar por un
// *PrincipalRegistry pelado se saltearía la delegación del envoltorio.
func registroRecargableDesde(t *testing.T, ruta, cuerpo, legacy string, mod time.Time) *reloadableRegistry {
	t.Helper()
	writeRegAt(t, ruta, cuerpo, mod)
	reg, err := loadPrincipals(ruta, legacy)
	if err != nil {
		t.Fatalf("no se pudo cargar el registro de prueba: %v", err)
	}
	fi, err := os.Stat(ruta)
	if err != nil {
		t.Fatal(err)
	}
	return newReloadableRegistry(ruta, legacy, reg, fi.ModTime())
}

// scrapear hace el GET a /metrics por HTTP de verdad, con el bearer dado, y devuelve el cuerpo.
func scrapear(t *testing.T, s *McpServer, reg principalResolver, bearer string) string {
	t.Helper()
	ts := httptest.NewServer(s.HTTPHandler(httpOptions{reqTimeout: 10 * time.Second, registry: reg}))
	defer ts.Close()
	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/metrics", nil)
	req.Header.Set("Authorization", "Bearer "+bearer)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /metrics: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /metrics con %q devolvió %d: la credencial de la prueba no autentica, y sin eso no se mide nada", bearer, resp.StatusCode)
	}
	crudo, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return string(crudo)
}

// muestra devuelve el valor de la muestra SIN etiquetas de una serie, y si estaba.
func muestra(cuerpo, nombre string) (string, bool) {
	for _, l := range strings.Split(cuerpo, "\n") {
		if v, ok := strings.CutPrefix(l, nombre+" "); ok {
			return strings.TrimSpace(v), true
		}
	}
	return "", false
}

// El registro de las pruebas de /metrics: una de cada estado, más una read=own que NO tiene que
// ver el bloque, y el bearer legacy encendido.
func registroDeVencimientos(ahora time.Time) string {
	f := func(d time.Duration) string { return ahora.Add(d).Format(time.RFC3339) }
	return `principals:
  - name: por-vencer
    token_sha256: ` + hashToken("tok-por-vencer") + `
    project_id: casa
    role: reader
    expires: "` + f(3*24*time.Hour) + `"
  - name: scrape
    token_sha256: ` + hashToken("tok-scrape") + `
    role: reader
    read: all
    expires: "` + f(100*24*time.Hour) + `"
  - name: vencida
    token_sha256: ` + hashToken("tok-vencida") + `
    project_id: casa
    role: reader
    expires: "` + f(-time.Hour) + `"
  - name: eterna
    token_sha256: ` + hashToken("tok-eterna") + `
    project_id: casa
    role: writer
  - name: acotada
    token_sha256: ` + hashToken("tok-acotada") + `
    project_id: casa
    role: reader
    expires: "` + f(200*24*time.Hour) + `"
`
}

// /METRICS DICE CUÁNTO FALTA PARA EL PRÓXIMO VENCIMIENTO, CUÁNTAS VENCIERON Y CUÁNTAS NO VENCEN.
//
// Es lo que leen las alertas CredencialPorVencer y CredencialRecienVencida. Se pide por HTTP de
// verdad, con la credencial de una identidad read=all —la forma del scrape—, sobre el registro
// RECARGABLE, que es el que corre en producción: una serie que existe en una función y no en la
// boca es la guarda definida y desconectada.
//
// Sabotaje que la pone roja: no llamar al render desde el handler de /metrics.
// arnes: archivo="internal/mcp/http.go"
// arnes: de="\t\ts.renderVencimientoDeCredenciales(&b, opt.registry, quien)\n"
// arnes: a="\t\t_ = opt.registry\n"
//
// Sabotaje que la pone roja: que la vencida entre al mínimo (el «próximo» sale negativo).
// arnes: archivo="internal/mcp/principals.go"
// arnes: de="\t\t\tres.Vencidas++\n\t\t\tcontinue\n"
// arnes: a="\t\t\tres.Vencidas++\n"
//
// Sabotaje que la pone roja: que el envoltorio recargable no delegue en su snapshot.
// arnes: archivo="internal/mcp/principals_reload.go"
// arnes: de="\t\tres = reg.resumenDeVencimientos(ahora)\n"
// arnes: a="\t\t_ = reg\n"
func TestMetricsDiceCuantoFaltaParaElProximoVencimiento(t *testing.T) {
	ahora := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	relojDeVencimiento(t, ahora)
	ruta := filepath.Join(t.TempDir(), "principals.yaml")
	rr := registroRecargableDesde(t, ruta, registroDeVencimientos(ahora), "token-legacy", time.Unix(1_000_000, 0))

	cuerpo := scrapear(t, newTestServer(t, embedding.NoopProvider{}), rr, "tok-scrape")

	quiere := map[string]string{
		// La de +3 d, y no la vencida de −1 h: una credencial muerta no es «la próxima en vencer».
		nombreProximoVencimiento:   "259200",
		nombreCredencialesVencidas: "1",
		nombreCredencialesEternas:  "1",
		nombreLegacyHabilitado:     "1",
		// El scrape entró con su propia credencial: el legacy no se usó.
		nombreLegacyAciertos:      "0",
		nombreRegistroSinRecargar: "0",
		nombreRegistroRecargasMal: "0",
	}
	for _, nombre := range seriesDeVencimiento {
		v, hay := muestra(cuerpo, nombre)
		if !hay {
			t.Errorf("/metrics no trae %s: la alerta que la lee no puede disparar nunca, y sin un solo error", nombre)
			continue
		}
		if v != quiere[nombre] {
			t.Errorf("%s = %s y tendría que ser %s", nombre, v, quiere[nombre])
		}
	}
	// NINGÚN NOMBRE DE PRINCIPAL: quién tiene acceso es un dato de admin.
	for _, n := range []string{"por-vencer", "vencida", "eterna", "acotada"} {
		if strings.Contains(cuerpo, `"`+n+`"`) {
			t.Errorf("/metrics nombra al principal %q: la lista de identidades es un dato sólo para admin", n)
		}
	}
}

// SIN FECHA FUTURA NO HAY «PRÓXIMO VENCIMIENTO», Y LA SERIE NO SALE.
//
// Hoy es el caso del central: ninguna credencial tiene `expires:`. Un 0 acá se leería como
// «vence en este instante» y `CredencialPorVencer` (`< 14 días`) dispararía en el acto, por un
// registro que está exactamente como se decidió. Ausente es «no hay nada que vencer».
//
// Sabotaje que la pone roja: emitir la muestra aunque no haya ninguna fecha futura.
// arnes: archivo="internal/mcp/principals_metricas.go"
// arnes: de="\tif r.HayProximo {\n\t\tfmt.Fprintf(b, \"%s %d\\n\", nombreProximoVencimiento"
// arnes: a="\tif r.HayProximo || true {\n\t\tfmt.Fprintf(b, \"%s %d\\n\", nombreProximoVencimiento"
func TestSinFechaFuturaNoHayProximoVencimiento(t *testing.T) {
	ruta := filepath.Join(t.TempDir(), "principals.yaml")
	rr := registroRecargableDesde(t, ruta, `principals:
  - name: scrape
    token_sha256: `+hashToken("tok-scrape")+`
    role: reader
    read: all
  - name: vieja
    token_sha256: `+hashToken("tok-vieja")+`
    project_id: casa
    role: reader
    expires: "2001-02-03T04:05:06Z"
`, "", time.Unix(1_000_000, 0))

	cuerpo := scrapear(t, newTestServer(t, embedding.NoopProvider{}), rr, "tok-scrape")
	if v, hay := muestra(cuerpo, nombreProximoVencimiento); hay {
		t.Errorf("%s = %s con ninguna credencial por vencer: la alerta `< 14 días` dispararía por un registro sano", nombreProximoVencimiento, v)
	}
	// Y el resto sí sale: la ausencia es de UNA serie, no del bloque.
	if v, _ := muestra(cuerpo, nombreCredencialesVencidas); v != "1" {
		t.Errorf("%s = %q y tendría que ser 1", nombreCredencialesVencidas, v)
	}
	if v, _ := muestra(cuerpo, nombreLegacyHabilitado); v != "0" {
		t.Errorf("%s = %q sin bearer legacy configurado; tendría que ser 0", nombreLegacyHabilitado, v)
	}
}

// UN LECTOR ACOTADO (read=own) NO VE EL BLOQUE.
//
// Sin nombres igual cuenta cosas —que hay un bearer admin que no vence, cuántas credenciales son
// eternas— y eso no es para un tercero que sólo ve su proyecto. El scrape es read=all y lo sigue
// recibiendo.
//
// Sabotaje que la pone roja: dejar que cualquier principal autenticado vea el bloque.
// arnes: archivo="internal/mcp/principals_metricas.go"
// arnes: de="\treturn read == ReadAll\n}"
// arnes: a="\treturn read == ReadAll || quien != nil\n}"
func TestUnLectorAcotadoNoVeElVencimientoDeLasCredenciales(t *testing.T) {
	ahora := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	relojDeVencimiento(t, ahora)
	ruta := filepath.Join(t.TempDir(), "principals.yaml")
	rr := registroRecargableDesde(t, ruta, registroDeVencimientos(ahora), "token-legacy", time.Unix(1_000_000, 0))
	s := newTestServer(t, embedding.NoopProvider{})

	// El control primero: la misma boca, con una identidad read=all, SÍ lo trae. Sin esto, un
	// render roto daría este verde de abajo por el motivo equivocado.
	if _, hay := muestra(scrapear(t, s, rr, "tok-scrape"), nombreCredencialesVencidas); !hay {
		t.Fatal("ni siquiera una identidad read=all ve el bloque: la prueba de abajo no mediría nada")
	}

	cuerpo := scrapear(t, s, rr, "tok-acotada")
	for _, nombre := range seriesDeVencimiento {
		if strings.Contains(cuerpo, nombre) {
			t.Errorf("un principal read=own ve %s en /metrics: el resumen del registro es para quien ve todo", nombre)
		}
	}
}

// UNA RECARGA RECHAZADA SE VE EN /METRICS, Y DEJA DE VERSE CUANDO EL ARCHIVO SE ARREGLA.
//
// Conservar el registro anterior ante un principals.yaml roto es correcto y silencioso: la
// revocación que estaba en el archivo no se aplica y el próximo reinicio no arranca, y hasta acá
// lo único que quedaba era un Warn en el journal.
//
// Sabotaje que la pone roja: no encender el flag en la rama del Warn.
// arnes: archivo="internal/mcp/principals_reload.go"
// arnes: de="\t\trr.recargaFallando.Store(true)\n"
// arnes: a="\t\t_ = rr.recargaFallando.Load()\n"
//
// Sabotaje que la pone roja: no apagarlo cuando la relectura vuelve a andar.
// arnes: archivo="internal/mcp/principals_reload.go"
// arnes: de="\trr.recargaFallando.Store(false)\n"
// arnes: a="\t_ = rr.recargaFallando.Load()\n"
//
// Sabotaje que la pone roja: no contar la relectura rechazada.
// arnes: archivo="internal/mcp/principals_reload.go"
// arnes: de="\t\trr.recargasFallidas.Add(1)\n"
// arnes: a="\t\t_ = rr.recargasFallidas.Load()\n"
func TestUnaRecargaRechazadaSeVeEnMetrics(t *testing.T) {
	ruta := filepath.Join(t.TempDir(), "principals.yaml")
	sano := `principals:
  - name: scrape
    token_sha256: ` + hashToken("tok-scrape") + `
    role: reader
    read: all
`
	t0 := time.Unix(3_000_000, 0)
	rr := registroRecargableDesde(t, ruta, sano, "", t0)
	s := newTestServer(t, embedding.NoopProvider{})

	leer := func(nombre string) string {
		t.Helper()
		v, hay := muestra(scrapear(t, s, rr, "tok-scrape"), nombre)
		if !hay {
			t.Fatalf("/metrics no trae %s", nombre)
		}
		return v
	}
	if v := leer(nombreRegistroSinRecargar); v != "0" {
		t.Fatalf("con el archivo sano %s = %s; tendría que ser 0", nombreRegistroSinRecargar, v)
	}

	// Una fecha ilegible: el registro de arranque la rechaza, así que la recarga también.
	writeRegAt(t, ruta, sano+`  - name: con-typo
    token_sha256: `+hashToken("tok-typo")+`
    project_id: casa
    role: reader
    expires: "el jueves que viene"
`, t0.Add(time.Minute))
	rr.reloadIfChanged()

	if v := leer(nombreRegistroSinRecargar); v != "1" {
		t.Errorf("con principals.yaml rechazado %s = %s: la revocación que estaba en el archivo no se aplicó y nada lo dice", nombreRegistroSinRecargar, v)
	}
	if v := leer(nombreRegistroRecargasMal); v != "1" {
		t.Errorf("%s = %s después de UNA relectura rechazada", nombreRegistroRecargasMal, v)
	}

	// Arreglado el archivo, el flag vuelve a 0 y el contador conserva la historia.
	writeRegAt(t, ruta, sano, t0.Add(2*time.Minute))
	rr.reloadIfChanged()
	if v := leer(nombreRegistroSinRecargar); v != "0" {
		t.Errorf("con el archivo arreglado %s sigue en %s: la alerta quedaría sonando por algo que ya se arregló", nombreRegistroSinRecargar, v)
	}
	if v := leer(nombreRegistroRecargasMal); v != "1" {
		t.Errorf("%s = %s: el contador es la historia y no puede volver atrás al arreglarse", nombreRegistroRecargasMal, v)
	}
}

// LOS USOS DEL BEARER LEGACY SE CUENTAN, Y LA CUENTA SOBREVIVE A LA RECARGA DEL REGISTRO.
//
// Es la cuenta que tiene que llegar a «siete días en cero» para retirar el legacy, y cuenta en
// resolve() porque por ahí pasan todas las puertas, incluidas las que el ledger de tools no ve.
// Cada edición de principals.yaml arma un registro nuevo: si la cuenta viviera sólo en el
// snapshot, revocar a cualquiera la pondría en cero.
//
// Sabotaje que la pone roja: no contar el acierto del legacy.
// arnes: archivo="internal/mcp/principals.go"
// arnes: de="\t\t\tr.legacyAciertos.Add(1)\n"
// arnes: a="\t\t\t_ = r.legacyAciertos.Load()\n"
//
// Sabotaje que la pone roja: no pasarle el contador al snapshot recargado.
// arnes: archivo="internal/mcp/principals_reload.go"
// arnes: de="\t\treg.legacyAciertos = prev.legacyAciertos\n"
// arnes: a="\t\t_ = prev.legacyAciertos\n"
func TestLosUsosDelLegacySeCuentanYSobrevivenALaRecarga(t *testing.T) {
	ruta := filepath.Join(t.TempDir(), "principals.yaml")
	uno := `principals:
  - name: scrape
    token_sha256: ` + hashToken("tok-scrape") + `
    role: reader
    read: all
`
	t0 := time.Unix(4_000_000, 0)
	rr := registroRecargableDesde(t, ruta, uno, "token-legacy", t0)

	if p, ok := rr.resolve("token-legacy"); !ok || p.Name != "legacy" {
		t.Fatalf("el bearer legacy no autenticó (%+v, %v): la prueba no mediría nada", p, ok)
	}

	// Una edición cualquiera del registro: alta de otro principal.
	writeRegAt(t, ruta, uno+`  - name: nuevo
    token_sha256: `+hashToken("tok-nuevo")+`
    project_id: casa
    role: reader
`, t0.Add(time.Minute))
	rr.reloadIfChanged()
	if _, ok := rr.resolve("tok-nuevo"); !ok {
		t.Fatal("la recarga no tomó el archivo nuevo: la prueba no estaría cruzando una recarga")
	}

	// El scrape entra CON el legacy: ése es el segundo uso, y se cuenta antes de renderizar.
	cuerpo := scrapear(t, newTestServer(t, embedding.NoopProvider{}), rr, "token-legacy")
	if v, _ := muestra(cuerpo, nombreLegacyAciertos); v != "2" {
		t.Errorf("%s = %q después de dos usos del legacy con una recarga en el medio; tendría que ser 2", nombreLegacyAciertos, v)
	}
}

// LAS ALERTAS DE CREDENCIALES LEEN SERIES QUE EL CEREBRO EMITE.
//
// Los nombres viven en dos lugares —las constantes de principals_metricas.go y las `expr` de
// deploy/musubi-alerts.yml— y renombrar uno sin el otro compila, pasa el `promtool check` y deja
// la alerta apagada para siempre: una `expr` sobre una serie que no existe no dispara ni falla.
//
// Sabotaje que la pone roja: renombrar una serie en el Go sin tocar la alerta que la lee.
// arnes: archivo="internal/mcp/principals_metricas.go"
// arnes: de="\tnombreRegistroSinRecargar  = \"musubi_principals_reload_failing\""
// arnes: a="\tnombreRegistroSinRecargar  = \"musubi_principals_reload_failed\""
func TestLasAlertasDeCredencialesLeenSeriesQueElCerebroEmite(t *testing.T) {
	ahora := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	relojDeVencimiento(t, ahora)
	ruta := filepath.Join(t.TempDir(), "principals.yaml")
	rr := registroRecargableDesde(t, ruta, registroDeVencimientos(ahora), "token-legacy", time.Unix(1_000_000, 0))
	cuerpo := scrapear(t, newTestServer(t, embedding.NoopProvider{}), rr, "tok-scrape")

	reglas, _ := cargarReglas(t, "musubi-alerts.yml")
	reSerie := regexp.MustCompile(`\bmusubi_[a-z_]+\b`)
	for _, alerta := range []string{"CredencialPorVencer", "CredencialRecienVencida", "RegistroDePrincipalsSinPoderRecargarse"} {
		expr, ok := exprDeAlerta(reglas, alerta)
		if !ok {
			t.Errorf("no existe la alerta %s en musubi-alerts.yml", alerta)
			continue
		}
		series := reSerie.FindAllString(expr, -1)
		if len(series) == 0 {
			t.Errorf("la expr de %s no lee ninguna serie musubi_*: %q", alerta, expr)
		}
		for _, serie := range series {
			if _, hay := muestra(cuerpo, serie); !hay {
				t.Errorf("%s lee %s y el cerebro no la emite en /metrics: la alerta no dispara nunca", alerta, serie)
			}
		}
	}
}
