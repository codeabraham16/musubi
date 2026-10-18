package mcp

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"musubi/internal/embedding"
)

// principals_uso_test.go cubre la SEÑAL DE USO de las identidades NOMBRADAS (auditoría-identidad):
// aciertos + last_used por principal, que hasta acá sólo tenía el bearer legacy (legacyAciertos).
// Es lo que deja decidir que un token está muerto y se puede retirar, por identidad y no en bloque.

// EL USO DE UN PRINCIPAL NOMBRADO SE CUENTA EN resolve(), Y LA HORA QUEDA SELLADA.
//
// Es el espejo por identidad de legacyAciertos: cuenta en resolve() porque por ahí pasan todas las
// puertas (incluidas las que el ledger de tools no ve), después del loop de ConstantTimeCompare
// para no tocar su tiempo constante.
//
// Sabotaje que la pone roja: no contar el acierto del principal nombrado.
// arnes: archivo="internal/mcp/principals.go"
// arnes: de="\t\tr.usos[strings.ToLower(match.Name)].registrar(ahoraParaVencimiento())\n"
// arnes: a="\t\t_ = match.Name\n"
func TestResolveCuentaElUsoDeCadaPrincipalNombrado(t *testing.T) {
	ahora := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	relojDeVencimiento(t, ahora)
	ruta := filepath.Join(t.TempDir(), "principals.yaml")
	writeRegAt(t, ruta, `principals:
  - name: mando
    token_sha256: `+hashToken("tok-mando")+`
    project_id: casa
    role: writer
`, time.Unix(1_000_000, 0))
	reg, err := loadPrincipals(ruta, "")
	if err != nil {
		t.Fatal(err)
	}

	// Antes de cualquier uso: en cero y sin fecha. Es el estado con el que /metrics lo cuenta como
	// «nunca usado» y token_list lo muestra «nunca»: tiene que nacer así, no inventado.
	if u := reg.usoDeIdentidades()["mando"]; u.Aciertos != 0 || u.UltimoUnix != 0 {
		t.Fatalf("un principal recién cargado arranca con uso %+v: la señal de retiro nacería mintiendo", u)
	}

	if _, ok := reg.resolve("tok-mando"); !ok {
		t.Fatal("tok-mando no autenticó: la prueba no estaría contando un uso real")
	}
	u := reg.usoDeIdentidades()["mando"]
	if u.Aciertos != 1 {
		t.Errorf("primer uso: aciertos = %d, tendría que ser 1", u.Aciertos)
	}
	if u.UltimoUnix != ahora.Unix() {
		t.Errorf("last_used = %d y el reloj del resolve marcaba %d: no se sella la hora del acierto", u.UltimoUnix, ahora.Unix())
	}

	if _, ok := reg.resolve("tok-mando"); !ok {
		t.Fatal("segundo resolve de tok-mando falló")
	}
	if u := reg.usoDeIdentidades()["mando"]; u.Aciertos != 2 {
		t.Errorf("segundo uso: aciertos = %d, tendría que ser 2", u.Aciertos)
	}
}

// EL USO SOBREVIVE A LA RECARGA EN CALIENTE, Y UN PRINCIPAL NUEVO NACE EN CERO.
//
// Cada edición de principals.yaml arma un registro nuevo (principals_reload.go). Si el uso viviera
// sólo en el snapshot, revocar o dar de alta a CUALQUIERA pondría en cero la cuenta de TODOS —y la
// señal «siete días sin usarse» no llegaría nunca a siete—. El carry por nombre lo evita.
//
// Sabotaje que la pone roja: no pasarle el uso al snapshot recargado.
// arnes: archivo="internal/mcp/principals_reload.go"
// arnes: de="\t\t\t\treg.usos[nombre] = u\n"
// arnes: a="\t\t\t\t_ = u\n"
func TestElUsoDeUnPrincipalSobreviveALaRecarga(t *testing.T) {
	ahora := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	relojDeVencimiento(t, ahora)
	ruta := filepath.Join(t.TempDir(), "principals.yaml")
	uno := `principals:
  - name: mando
    token_sha256: ` + hashToken("tok-mando") + `
    project_id: casa
    role: writer
`
	t0 := time.Unix(4_000_000, 0)
	rr := registroRecargableDesde(t, ruta, uno, "", t0)

	if _, ok := rr.resolve("tok-mando"); !ok {
		t.Fatal("tok-mando no autenticó antes de la recarga")
	}
	if u := rr.usoDeIdentidades()["mando"]; u.Aciertos != 1 {
		t.Fatalf("aciertos = %d antes de la recarga; tendría que ser 1", u.Aciertos)
	}

	// Alta de otro principal, con mtime posterior: la recarga arma un registro NUEVO.
	writeRegAt(t, ruta, uno+`  - name: nuevo
    token_sha256: `+hashToken("tok-nuevo")+`
    project_id: casa
    role: reader
`, t0.Add(time.Minute))
	rr.reloadIfChanged()

	// El recién llegado nace en cero: no hereda el uso de nadie.
	if u := rr.usoDeIdentidades()["nuevo"]; u.Aciertos != 0 || u.UltimoUnix != 0 {
		t.Errorf("el principal nuevo arranca con uso %+v; tendría que nacer en cero", u)
	}
	if _, ok := rr.resolve("tok-nuevo"); !ok {
		t.Fatal("la recarga no tomó el archivo nuevo: la prueba no estaría cruzando una recarga")
	}

	// mando se vuelve a usar: el segundo acierto se SUMA al que sobrevivió la recarga.
	if _, ok := rr.resolve("tok-mando"); !ok {
		t.Fatal("tok-mando no autenticó tras la recarga")
	}
	if u := rr.usoDeIdentidades()["mando"]; u.Aciertos != 2 {
		t.Errorf("aciertos de mando = %d tras una recarga en el medio; tendría que ser 2 (el uso no sobrevivió)", u.Aciertos)
	}
}

// EL ALTA REGISTRA QUIÉN LA MINTEÓ Y UN IDENTIFICADOR CORTO DEL TOKEN (auditoría de creación).
//
// created_by = el actor; por la red es el admin que la minteó, por CLI local es "cli". token_prefix
// es un identificador para parear la credencial con la que alguien tiene a mano —NO el secreto—.
//
// Sabotaje que la pone roja: ignorar el actor y marcar siempre "cli".
// arnes: archivo="internal/mcp/principals_admin.go"
// arnes: de="\t\te.CreatedBy = a\n"
// arnes: a="\t\te.CreatedBy = \"cli\"\n"
func TestElAltaRegistraElActorYElPrefijo(t *testing.T) {
	ruta := filepath.Join(t.TempDir(), "principals.yaml")

	// Alta por la red (MCP): el actor es el admin que la firmó.
	tok, err := AddPrincipalWithCaps(ruta, "porred", "casa", RoleWriter, ReadAll, WriteOwn, "admin-ana")
	if err != nil {
		t.Fatal(err)
	}
	// Alta por CLI local: AddPrincipal pasa "cli".
	if _, err := AddPrincipal(ruta, "porcli", "casa", "writer"); err != nil {
		t.Fatal(err)
	}

	infos, err := ListPrincipalsInfo(ruta)
	if err != nil {
		t.Fatal(err)
	}
	porNombre := map[string]PrincipalInfo{}
	for _, p := range infos {
		porNombre[p.Name] = p
	}
	red, cli := porNombre["porred"], porNombre["porcli"]

	if red.CreatedBy != "admin-ana" {
		t.Errorf("created_by del alta por red = %q; tendría que ser el admin que la minteó", red.CreatedBy)
	}
	if cli.CreatedBy != "cli" {
		t.Errorf("created_by del alta por CLI = %q; tendría que ser \"cli\"", cli.CreatedBy)
	}
	if _, err := time.Parse(time.RFC3339, red.CreatedAt); err != nil {
		t.Errorf("created_at = %q no es RFC3339: %v", red.CreatedAt, err)
	}
	if red.TokenPrefix == "" || !strings.HasPrefix(tok, red.TokenPrefix) {
		t.Errorf("token_prefix %q no es prefijo del token entregado: no sirve para parear", red.TokenPrefix)
	}
	if len(red.TokenPrefix) > 8 {
		t.Errorf("token_prefix %q mide %d: un identificador no lleva tanto del token", red.TokenPrefix, len(red.TokenPrefix))
	}
}

// UN REGISTRO DEL FORMATO ANTERIOR CARGA SIN ERROR, Y EL PREFIJO NO ES UNA LLAVE.
//
// Los campos de auditoría son omitempty: un principals.yaml escrito por un binario anterior (sin
// created_at/created_by/token_prefix) sigue cargando y autenticando, y los campos quedan vacíos,
// no inventados. Y token_prefix es un IDENTIFICADOR: resolve() lo niega como credencial.
func TestRegistroViejoCargaYElPrefijoNoEsLlave(t *testing.T) {
	// Backward-compat: formato anterior, sin los campos nuevos.
	ruta := filepath.Join(t.TempDir(), "principals.yaml")
	writeRegAt(t, ruta, `principals:
  - name: viejo
    token_sha256: `+hashToken("tok-viejo-sin-campos")+`
    project_id: casa
    role: writer
`, time.Unix(1_000_000, 0))
	reg, err := loadPrincipals(ruta, "")
	if err != nil {
		t.Fatalf("un registro del formato anterior no carga: %v", err)
	}
	if _, ok := reg.resolve("tok-viejo-sin-campos"); !ok {
		t.Fatal("el token de un registro viejo dejó de autenticar")
	}
	infos, err := ListPrincipalsInfo(ruta)
	if err != nil {
		t.Fatal(err)
	}
	if infos[0].CreatedBy != "" || infos[0].TokenPrefix != "" || infos[0].CreatedAt != "" {
		t.Errorf("un registro viejo trae auditoría inventada: %+v", infos[0])
	}

	// El prefijo NO es una llave: se mintea un principal real, se toma su token_prefix GUARDADO, y
	// resolve lo niega.
	ruta2 := filepath.Join(t.TempDir(), "principals.yaml")
	tok, err := AddPrincipalWithCaps(ruta2, "real", "casa", RoleWriter, ReadAll, WriteOwn, "admin")
	if err != nil {
		t.Fatal(err)
	}
	reg2, err := loadPrincipals(ruta2, "")
	if err != nil {
		t.Fatal(err)
	}
	infos2, err := ListPrincipalsInfo(ruta2)
	if err != nil {
		t.Fatal(err)
	}
	pref := infos2[0].TokenPrefix
	if pref == "" {
		t.Fatal("el principal minteado no guardó token_prefix: la prueba de abajo no separaría prefijo de token")
	}
	// Control: el token COMPLETO sí autentica. Sin esto, un resolve roto daría el verde de abajo
	// por el motivo equivocado.
	if _, ok := reg2.resolve(tok); !ok {
		t.Fatal("el token completo no autenticó: resolve está roto y la prueba no mide el prefijo")
	}
	if _, ok := reg2.resolve(pref); ok {
		t.Error("el token_prefix autenticó: un identificador corto NO puede ser una credencial")
	}
}

// /METRICS CUENTA LOS PRINCIPALS QUE NUNCA SE USARON, SIN NOMBRARLOS.
//
// Es la contracara nombrada del legacy: musubi_principals_never_used dice CUÁNTOS candidatos a
// retirar hay, sin decir cuáles —la lista de identidades es un dato de admin (toolTokenList)—. El
// legacy no cuenta acá: usoDeIdentidades sólo cubre los principals del registro.
//
// Sabotaje que la pone roja: contar los USADOS en vez de los que nunca se usaron.
// arnes: archivo="internal/mcp/principals_metricas.go"
// arnes: de="\t\tif u.UltimoUnix == 0 {\n"
// arnes: a="\t\tif u.UltimoUnix != 0 {\n"
func TestMetricsCuentaLosPrincipalsNuncaUsadosSinNombrarlos(t *testing.T) {
	ahora := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	relojDeVencimiento(t, ahora)
	ruta := filepath.Join(t.TempDir(), "principals.yaml")
	// Tres nombrados: el que scrapea (read=all) y dos dormidos que nunca autenticaron.
	rr := registroRecargableDesde(t, ruta, `principals:
  - name: scrape
    token_sha256: `+hashToken("tok-scrape")+`
    role: reader
    read: all
  - name: dormido-uno
    token_sha256: `+hashToken("tok-d1")+`
    project_id: casa
    role: reader
  - name: dormido-dos
    token_sha256: `+hashToken("tok-d2")+`
    project_id: casa
    role: reader
`, "", time.Unix(1_000_000, 0))
	s := newTestServer(t, embedding.NoopProvider{})

	// El scrape entra con su credencial: queda USADO. Los dos dormidos nunca autenticaron ⇒ 2.
	cuerpo := scrapear(t, s, rr, "tok-scrape")
	v, hay := muestra(cuerpo, nombreNuncaUsadas)
	if !hay {
		t.Fatalf("/metrics no trae %s: la señal de retiro no se publica", nombreNuncaUsadas)
	}
	if v != "2" {
		t.Errorf("%s = %s con dos nombrados sin usar (y scrape ya usado); tendría que ser 2", nombreNuncaUsadas, v)
	}
	// SIN NOMBRES: cuáles son los dormidos es un dato de admin, no de quien scrapea /metrics.
	for _, n := range []string{"dormido-uno", "dormido-dos"} {
		if strings.Contains(cuerpo, `"`+n+`"`) {
			t.Errorf("/metrics nombra al principal %q: la señal de retiro cuenta, no identifica", n)
		}
	}
}

// EL CONTEO DE USO AGUANTA resolve() CONCURRENTE CRUZADO POR UNA RECARGA (R2 y D5).
//
// resolve() incrementa SIN tomar locks: el mapa `usos` es inmutable tras loadPrincipals y sólo se
// mutan los átomos de cada usoStat; la recarga publica un snapshot nuevo con atomic.Pointer y
// CONSERVA el puntero del usoStat por nombre (principals_reload.go). Si algo de eso estuviera mal
// —un acceso al mapa sin sincronizar, o un carry que pierde el puntero— este test lo delata de dos
// maneras independientes: con -race, el detector marca el acceso; y la cuenta final no da el total
// exacto de autenticaciones, porque un incremento caería en un usoStat que ningún snapshot publicado
// vuelve a ver.
//
// No lleva sabotaje del arnes a propósito: lo que mide es la AUSENCIA de carrera, que no se prueba
// rompiendo una línea sino corriendo con -race. El carry, que SÍ es una línea, ya lo cubre el
// sabotaje de TestElUsoDeUnPrincipalSobreviveALaRecarga.
func TestResolveYRecargaConcurrentesNoPierdenCuenta(t *testing.T) {
	relojDeVencimiento(t, time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC))
	ruta := filepath.Join(t.TempDir(), "principals.yaml")
	yaml := `principals:
  - name: mando
    token_sha256: ` + hashToken("tok-mando") + `
    project_id: casa
    role: writer
`
	t0 := time.Unix(5_000_000, 0)
	rr := registroRecargableDesde(t, ruta, yaml, "", t0)

	const G, K = 24, 200 // 4800 autenticaciones, todas sobre el MISMO usoStat (lo conserva el carry)
	const reloads = 40
	var wg sync.WaitGroup

	// Recargas en el medio: reescriben el MISMO contenido con mtime creciente, forzando
	// loadPrincipals + carry mientras las goroutines autentican. No toca `t` (ilegal fuera de la
	// goroutine de test); los errores de disco son transitorios y el peor caso es una recarga menos.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 1; i <= reloads; i++ {
			mt := t0.Add(time.Duration(i) * time.Second)
			_ = os.WriteFile(ruta, []byte(yaml), 0o600)
			_ = os.Chtimes(ruta, mt, mt)
			rr.reloadIfChanged()
		}
	}()

	for g := 0; g < G; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for k := 0; k < K; k++ {
				if _, ok := rr.resolve("tok-mando"); !ok {
					t.Error("tok-mando no autenticó durante la carrera: la recarga dejó un hueco")
					return
				}
			}
		}()
	}
	wg.Wait()

	if u := rr.usoDeIdentidades()["mando"]; u.Aciertos != G*K {
		t.Errorf("aciertos = %d tras %d autenticaciones cruzadas por %d recargas; se perdió la cuenta de %d",
			u.Aciertos, G*K, reloads, G*K-int(u.Aciertos))
	}
}

// EL TOOL token_list EXPONE EL USO VIVO DE CADA IDENTIDAD, Y UN DORMIDO SE VE «nunca».
//
// Es la hermana por USO de TestElToolTokenListDiceQueLaCredencialVencio: las dos cosas que token_list
// tiene que decir sobre una credencial —si venció y si alguien la usa— salen por la MISMA boca del
// tool, y la capa de abajo (usoDeIdentidades) puede seguir contando perfecto mientras esta boca tira
// las claves. El único test del tool corría con buscarPrincipal=nil (todo en cero): la rama de uso
// vivo —el RFC3339 de last_used y el aciertos>0— no la ejercía nadie. Acá se ejercita de verdad.
//
// Sabotaje que la pone roja: que el tool deje de copiar aciertos/last_used a la fila.
// arnes: archivo="internal/mcp/methods_admin_tokens.go"
// arnes: de="\t\tu := usos[strings.ToLower(p.Name)]\n\t\tfila[\"aciertos\"] = u.Aciertos\n\t\tif u.UltimoUnix > 0 {\n\t\t\tfila[\"last_used\"] = time.Unix(u.UltimoUnix, 0).UTC().Format(time.RFC3339)\n\t\t} else {\n\t\t\tfila[\"last_used\"] = \"nunca\"\n\t\t}\n"
// arnes: a="\t\t_ = usos[strings.ToLower(p.Name)]\n\t\t_ = time.RFC3339\n"
func TestElToolTokenListExponeElUsoVivoDeCadaIdentidad(t *testing.T) {
	ahora := time.Date(2026, 9, 10, 15, 30, 0, 0, time.UTC)
	relojDeVencimiento(t, ahora)
	ruta := filepath.Join(t.TempDir(), "principals.yaml")
	writeRegAt(t, ruta, `principals:
  - name: mando
    token_sha256: `+hashToken("tok-mando")+`
    project_id: casa
    role: writer
  - name: dormido
    token_sha256: `+hashToken("tok-dormido")+`
    project_id: casa
    role: reader
`, time.Unix(1_000_000, 0))
	reg, err := loadPrincipals(ruta, "")
	if err != nil {
		t.Fatal(err)
	}

	// mando autentica dos veces; dormido nunca. El listado tiene que distinguir una identidad usada
	// de una que jamás presentó credencial: es la decisión de retirar un token que nadie usa.
	for i := 0; i < 2; i++ {
		if _, ok := reg.resolve("tok-mando"); !ok {
			t.Fatal("tok-mando no autenticó: la prueba no estaría listando un uso real")
		}
	}

	s := &McpServer{principalsFile: ruta, buscarPrincipal: reg}
	ctx := withPrincipal(context.Background(), &Principal{Name: "jefa", Role: RoleAdmin, ProjectID: "casa"})
	res, rpcErr := s.toolTokenList(ctx, nil)
	if rpcErr != nil {
		t.Fatalf("toolTokenList: %v", rpcErr)
	}
	resp, ok := res.(CallToolResponse)
	if !ok || len(resp.Content) == 0 {
		t.Fatalf("toolTokenList devolvió %T sin contenido: %#v", res, res)
	}
	var payload struct {
		Principals []map[string]interface{} `json:"principals"`
	}
	if err := json.Unmarshal([]byte(resp.Content[0].Text), &payload); err != nil {
		t.Fatalf("el resultado del tool no es JSON parseable (%v): %s", err, resp.Content[0].Text)
	}
	filas := map[string]map[string]interface{}{}
	for _, p := range payload.Principals {
		nombre, _ := p["name"].(string)
		filas[nombre] = p
	}

	// mando: dos aciertos y la hora del último sellada. La CLAVE tiene que estar presente —ausente
	// se leería como 0/"" y "" sería «nunca» disfrazado de medición, justo el bug que esto custodia.
	mando := filas["mando"]
	if mando == nil {
		t.Fatal("el tool no devolvió a mando")
	}
	if _, hay := mando["aciertos"]; !hay {
		t.Error("mando: el tool no devuelve la clave \"aciertos\": el uso vivo es invisible para quien lista")
	} else if a, _ := mando["aciertos"].(float64); a != 2 {
		t.Errorf("mando: aciertos=%v y autenticó 2 veces", mando["aciertos"])
	}
	if _, hay := mando["last_used"]; !hay {
		t.Error("mando: el tool no devuelve la clave \"last_used\": no se sabe cuándo se usó por última vez")
	} else if lu, _ := mando["last_used"].(string); lu != ahora.Format(time.RFC3339) {
		t.Errorf("mando: last_used=%v y el reloj del acierto marcaba %s", mando["last_used"], ahora.Format(time.RFC3339))
	}

	// dormido: nunca autenticó ⇒ en cero y «nunca», no inventado. Es el estado con el que se decide
	// retirar un token que nadie usa; un "" acá sería «no pude medir» leído como «medí y es cero».
	dormido := filas["dormido"]
	if dormido == nil {
		t.Fatal("el tool no devolvió a dormido")
	}
	if a, _ := dormido["aciertos"].(float64); a != 0 {
		t.Errorf("dormido: aciertos=%v y nunca autenticó", dormido["aciertos"])
	}
	if lu, _ := dormido["last_used"].(string); lu != "nunca" {
		t.Errorf("dormido: last_used=%v y nunca autenticó (debería ser \"nunca\")", dormido["last_used"])
	}
}
