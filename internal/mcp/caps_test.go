package mcp

import (
	"os"
	"path/filepath"
	"testing"
)

// ALCANCE (qué VE) y AUTORIDAD (qué ESCRIBE) son ejes INDEPENDIENTES. El enum de roles los tenía
// colapsados, y por eso no sabía expresar las dos identidades que un cerebro central necesita:
//
//	SALA DE MANDO (el repo de Musubi): ve TODO —para diagnosticar los demás proyectos— pero
//	  escribe SÓLO en lo suyo. Con el enum había que darle `admin`, que además lo dejaba escribir
//	  dentro de la memoria de producción de cualquier otro proyecto.
//	CABINA (el CRM, el gateway): ve TODO y NO escribe NADA. Con el enum, `reader` no veía más que
//	  su propio tenant y `admin` escribía en todos: no había término medio.

// LA TRAMPA DEL CERO: el valor cero de un string es "", así que un Principal construido a mano
// —en un test, o en cualquier código que no pase por el registro— tiene Read/Write vacíos. Si el
// comportamiento los leyera directo, un reader podría MUTAR (Write != WriteNone) y un admin
// dejaría de ser federado (Read != ReadAll). Silencioso y catastrófico. El fallback al rol lo
// cierra: un Principal sin capacidades declaradas se comporta EXACTAMENTE como dice su rol.
func TestPrincipalSinCapsDeclaradasSeComportaSegunSuRol(t *testing.T) {
	reader := &Principal{Name: "r", ProjectID: "crm", Role: RoleReader} // sin Read/Write
	if reader.canCall(false) {
		t.Error("un reader sin caps declaradas NO puede mutar (el cero del string no puede decidir)")
	}

	admin := &Principal{Name: "a", Role: RoleAdmin} // sin Read/Write
	if _, federado := recallScopeFor(admin); !federado {
		t.Error("un admin sin caps declaradas DEBE seguir siendo federado")
	}
	if origin, ok := writeOriginFor(admin, "altura"); !ok || origin != "altura" {
		t.Errorf("un admin sin caps declaradas debe respetar el proyecto declarado: origin=%q ok=%v", origin, ok)
	}

	writer := &Principal{Name: "w", ProjectID: "musubi", Role: RoleWriter} // sin Read/Write
	if scope, federado := recallScopeFor(writer); federado || scope != "musubi" {
		t.Errorf("un writer sin caps declaradas ve SOLO lo suyo: scope=%q federado=%v", scope, federado)
	}
}

// La tabla de compat: cada rol histórico significa exactamente lo mismo que antes.
func TestCapsFromRoleEsLaTablaDeCompat(t *testing.T) {
	casos := []struct{ role, read, write string }{
		{RoleReader, ReadOwn, WriteNone},
		{RoleWriter, ReadOwn, WriteOwn},
		{RoleAdmin, ReadAll, WriteAny},
	}
	for _, c := range casos {
		r, w := capsFromRole(c.role)
		if r != c.read || w != c.write {
			t.Errorf("rol %s = (%s,%s), esperaba (%s,%s)", c.role, r, w, c.read, c.write)
		}
	}
}

// SALA DE MANDO: ve todos los proyectos, pero su escritura se clava en el suyo — aunque declare
// otro. Es la garantía de que Musubi puede diagnosticar el CRM sin poder plantarle memoria.
func TestSalaDeMandoVeTodoPeroEscribeSoloLoSuyo(t *testing.T) {
	mando := &Principal{Name: "davantis-mando", ProjectID: "musubi", Role: RoleWriter, Read: ReadAll, Write: WriteOwn}

	if _, federado := recallScopeFor(mando); !federado {
		t.Error("la sala de mando debe VER TODO (recall federado)")
	}
	origin, ok := writeOriginFor(mando, "crm") // intenta escribir en el tenant del CRM
	if !ok || origin != "musubi" {
		t.Errorf("la sala de mando declaró project=crm y quedó origin=%q (ok=%v); debe clavarse en 'musubi'", origin, ok)
	}
	if !mando.canCall(false) {
		t.Error("la sala de mando SÍ puede mutar (lo suyo)")
	}
}

// CABINA (CRM, gateway): ve todos los proyectos y no puede mutar ninguno. No hace falta que tenga
// proyecto propio: no escribe.
func TestCabinaVeTodoYNoEscribeNada(t *testing.T) {
	cabina := &Principal{Name: "crm-cabina", Role: RoleReader, Read: ReadAll, Write: WriteNone}

	if _, federado := recallScopeFor(cabina); !federado {
		t.Error("la cabina debe VER TODO (recall federado)")
	}
	if cabina.canCall(false) {
		t.Error("la cabina NO puede invocar tools que mutan")
	}
	if !cabina.canCall(true) {
		t.Error("la cabina SÍ puede invocar tools de lectura")
	}
}

// Un writer normal sigue acotado a lo suyo, en los dos ejes. El cambio no puede aflojarle nada.
func TestWriterNormalSigueAcotado(t *testing.T) {
	w := &Principal{Name: "davantis-crm", ProjectID: "crm", Role: RoleWriter, Read: ReadOwn, Write: WriteOwn}

	scope, federado := recallScopeFor(w)
	if federado || scope != "crm" {
		t.Errorf("un writer debe ver SOLO lo suyo: scope=%q federado=%v", scope, federado)
	}
	if origin, ok := writeOriginFor(w, "musubi"); !ok || origin != "crm" {
		t.Errorf("un writer que declara otro proyecto debe caer en el suyo: origin=%q ok=%v", origin, ok)
	}
}

// LA FUGA QUE ESTO CIERRA: una escritura sin proyecto queda SIN ATRIBUIR, y una fila sin atribuir
// la ven TODOS los tenants (el filtro de recall deja pasar project_id vacío). Medido en el cerebro
// real: 2 filas de test contaminando los 3 proyectos. Ahora se rechaza en vez de pasar en silencio.
func TestEscrituraSinProyectoSeRechaza(t *testing.T) {
	// write=any SIN proyecto propio y SIN declarar ninguno: antes caía en "" (sin atribuir).
	suelto := &Principal{Name: "legacy", Role: RoleAdmin, Read: ReadAll, Write: WriteAny}
	if _, ok := writeOriginFor(suelto, ""); ok {
		t.Error("una escritura sin proyecto declarado NI propio debe RECHAZARSE (si no, la fila la ven todos los tenants)")
	}
	// Pero declarando el proyecto, escribe donde dice: es el ingest del central y la reparación.
	if origin, ok := writeOriginFor(suelto, "altura"); !ok || origin != "altura" {
		t.Errorf("write=any DECLARANDO proyecto debe respetarlo: origin=%q ok=%v", origin, ok)
	}
}

// stdio local (sin principal) conserva la confianza local: acceso pleno, el engine estampa su
// propio project_id. No se puede romper este camino, es el del daemon en la máquina del usuario.
func TestStdioLocalSinPrincipalConservaAccesoPleno(t *testing.T) {
	if _, federado := recallScopeFor(nil); federado {
		t.Error("stdio local: sin scope, comportamiento histórico")
	}
	if !(*Principal)(nil).canCall(false) {
		t.Error("stdio local debe poder mutar")
	}
	if origin, ok := writeOriginFor(nil, "lo-que-diga-el-engine"); !ok || origin != "lo-que-diga-el-engine" {
		t.Errorf("stdio local: el origen lo pone el engine; origin=%q ok=%v", origin, ok)
	}
}

// El registro en YAML: read/write ausentes ⇒ se derivan del rol (compat); presentes ⇒ mandan.
func TestLoadPrincipalsDerivaYRespetaLosEjes(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "principals.yaml")
	yaml := `principals:
  - name: davantis-crm
    token_sha256: ` + hashToken("tok-crm") + `
    project_id: crm
    role: writer
  - name: davantis-mando
    token_sha256: ` + hashToken("tok-mando") + `
    project_id: musubi
    role: writer
    read: all
    write: own
  - name: crm-cabina
    token_sha256: ` + hashToken("tok-cabina") + `
    role: reader
    read: all
    write: none
`
	if err := os.WriteFile(path, []byte(yaml), 0600); err != nil {
		t.Fatal(err)
	}
	reg, err := loadPrincipals(path, "")
	if err != nil {
		t.Fatal(err)
	}

	p, ok := reg.resolve("tok-crm")
	if !ok || p.Read != ReadOwn || p.Write != WriteOwn {
		t.Errorf("sin read/write declarados debe derivar del rol writer: %+v", p)
	}
	p, ok = reg.resolve("tok-mando")
	if !ok || p.Read != ReadAll || p.Write != WriteOwn {
		t.Errorf("la sala de mando declarada en el YAML: %+v", p)
	}
	p, ok = reg.resolve("tok-cabina")
	if !ok || p.Read != ReadAll || p.Write != WriteNone {
		t.Errorf("la cabina declarada en el YAML (sin project_id, porque no escribe): %+v", p)
	}
}

// Fail-closed en el registro: quien ESCRIBE lo suyo debe TENER lo suyo. Sin project_id, su fila
// caería sin atribuir y la verían todos los tenants.
func TestRegistroRechazaWriteOwnSinProyecto(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "principals.yaml")
	yaml := `principals:
  - name: roto
    token_sha256: ` + hashToken("tok-roto") + `
    role: reader
    read: all
    write: own
`
	if err := os.WriteFile(path, []byte(yaml), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadPrincipals(path, ""); err == nil {
		t.Error("un principal con write=own y sin project_id debe RECHAZARSE al cargar el registro")
	}
}

// Y el alta por CLI espeja la misma guarda (defensa en profundidad: el YAML se puede editar a mano).
func TestAddPrincipalWithCapsGuardaYPersiste(t *testing.T) {
	path := filepath.Join(t.TempDir(), "principals.yaml")

	if _, err := AddPrincipalWithCaps(path, "roto", "", RoleWriter, ReadAll, WriteOwn); err == nil {
		t.Error("write=own sin --project debe rechazarse")
	}
	if _, err := AddPrincipalWithCaps(path, "mando", "musubi", RoleWriter, ReadAll, WriteOwn); err != nil {
		t.Fatalf("la sala de mando es válida: %v", err)
	}
	// Una cabina NO necesita proyecto: no escribe.
	if _, err := AddPrincipalWithCaps(path, "cabina", "", RoleReader, ReadAll, WriteNone); err != nil {
		t.Fatalf("la cabina no necesita proyecto (no escribe): %v", err)
	}

	infos, err := ListPrincipalsInfo(path)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string][2]string{}
	for _, i := range infos {
		got[i.Name] = [2]string{i.Read, i.Write}
	}
	if got["mando"] != [2]string{ReadAll, WriteOwn} {
		t.Errorf("mando = %v", got["mando"])
	}
	if got["cabina"] != [2]string{ReadAll, WriteNone} {
		t.Errorf("cabina = %v", got["cabina"])
	}
}
