package main

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"musubi/internal/skills"
)

// agentskills.go — escribe las skills de Musubi al formato que el AGENTE lee
// (`.claude/skills/<name>/SKILL.md`).
//
// POR QUÉ: medido, `musubi_resolve_skills` tuvo CERO llamadas en 30 días (ledger local y central),
// ningún hook menciona skills y `.claude/skills/` estaba vacío mientras `.musubi/skills/` tenía 11.
// El arsenal existía y nada lo aplicaba. Esto lo conecta con el único consumidor que ya está andando.
//
// `.claude/` está gitignored: lo que se escribe acá es estado LOCAL derivado, no algo a versionar.

// dirSkillsAgente es donde el agente busca sus skills. La ruta se verificó contra una skill que YA
// funciona en esta máquina, no contra la documentación. Vive en internal/skills porque el servidor
// también la lee (para nombrarle al agente sólo las skills que puede cargar).
const dirSkillsAgente = skills.DirSkillsAgente

// ReporteExport dice qué pasó, y en particular qué NO se tocó. Un export que sólo cuenta éxitos
// esconde justamente el caso que importa: el archivo que alguien editó y Musubi preservó.
type ReporteExport struct {
	Escritas    []string // nuevas o refrescadas
	Preservadas []string // editadas a mano: Musubi no las toca
	Retiradas   []string // ya no existen en el origen y estaban intactas
	Omitidas    []string // su nombre no es un slug: exportarlas escribiría fuera de .claude/skills
}

// exportarSkillsAlAgente vuelca las skills de .musubi/skills/ a .claude/skills/<name>/SKILL.md.
//
// Exporta TODAS, no sólo las cognitivas: el arsenal que bajó del central es justamente el que se
// quiere poner a trabajar.
func exportarSkillsAlAgente(root string) (ReporteExport, error) {
	var rep ReporteExport
	resolver := skills.NewResolver(root)
	arsenal, err := resolver.LoadSkills()
	if err != nil {
		return rep, err
	}

	destino := filepath.Join(root, filepath.FromSlash(dirSkillsAgente))
	if err := os.MkdirAll(destino, 0o755); err != nil {
		return rep, err
	}

	vivas := make(map[string]bool, len(arsenal))
	for _, sk := range arsenal {
		// Un nombre que no es slug no se exporta. El destino es Join(destino, sk.Name), y LoadSkills
		// sólo exige que el nombre no esté vacío: un yaml con `name: ../../../fuera` —que puede llegar
		// por un pull, porque .musubi/skills/ viaja por git en otros repos— escribía un SKILL.md fuera
		// del proyecto, y en Windows alcanzaba para instalar una skill GLOBAL del usuario. Con el
		// refresco del arranque eso pasaba solo, sin que nadie corriera nada. La regla de slug existía,
		// pero sólo en la puerta MCP; lo que llega por git o a mano no pasaba por ahí.
		if !skills.NombreSeguro(sk.Name) {
			rep.Omitidas = append(rep.Omitidas, sk.Name)
			continue
		}
		vivas[sk.Name] = true
		accion, err := escribirSkillMD(destino, sk)
		if err != nil {
			return rep, err
		}
		switch accion {
		case "escrita":
			rep.Escritas = append(rep.Escritas, sk.Name)
		case "preservada":
			rep.Preservadas = append(rep.Preservadas, sk.Name)
		}
	}

	retiradas, err := retirarHuerfanas(destino, vivas)
	if err != nil {
		return rep, err
	}
	rep.Retiradas = retiradas

	sort.Strings(rep.Escritas)
	sort.Strings(rep.Preservadas)
	sort.Strings(rep.Retiradas)
	sort.Strings(rep.Omitidas)
	return rep, nil
}

// escribirSkillMD escribe una skill, o preserva el archivo si alguien lo editó.
// Devuelve "escrita", "preservada" o "sin-cambios".
func escribirSkillMD(destino string, sk skills.Skill) (string, error) {
	sum, err := skills.ChecksumSkillMD(sk)
	if err != nil {
		return "", err
	}
	contenido, err := skills.ASkillMD(sk, sum)
	if err != nil {
		return "", err
	}

	dir := filepath.Join(destino, sk.Name)
	ruta := filepath.Join(dir, "SKILL.md")

	if previo, err := os.ReadFile(ruta); err == nil {
		// REGLA DE ORO, la misma que managedSkillAction: ante la mínima duda, preservar. Un archivo
		// sin checksum de Musubi NO es de Musubi —puede ser una skill que el usuario instaló a mano
		// con el mismo nombre— y uno con checksum que no cuadra fue editado.
		if !skills.SigueIntacto(previo) {
			return "preservada", nil
		}
		// IDEMPOTENCIA: sin cambios reales no se toca el archivo. Sin esto, cada `setup` reescribiría
		// todo y ensuciaría cualquier diff o watcher.
		if strings.ReplaceAll(string(previo), "\r", "") == contenido {
			return "sin-cambios", nil
		}
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	if err := escribirArchivoAtomico(ruta, []byte(contenido), 0o644); err != nil {
		return "", err
	}
	return "escrita", nil
}

// retirarHuerfanas borra los SKILL.md que Musubi escribió y cuyo origen ya no existe.
//
// Existe por un caso real y documentado: `starter.yaml` vivió dos meses en cinco repos, idéntica
// byte a byte, generada por una versión vieja de `setup` que ya no la producía. Una skill huérfana
// que nadie mantiene sigue costando contexto en cada turno.
//
// SÓLO borra lo que sigue INTACTO: si alguien la editó, cae en la regla de oro y sobrevive.
func retirarHuerfanas(destino string, vivas map[string]bool) ([]string, error) {
	entradas, err := os.ReadDir(destino)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, e := range entradas {
		if !e.IsDir() || vivas[e.Name()] {
			continue
		}
		ruta := filepath.Join(destino, e.Name(), "SKILL.md")
		contenido, err := os.ReadFile(ruta)
		if err != nil {
			continue // no es una skill nuestra: no tocar
		}
		if !skills.SigueIntacto(contenido) {
			continue // editada a mano: sobrevive
		}
		if err := os.RemoveAll(filepath.Join(destino, e.Name())); err != nil {
			return out, err
		}
		out = append(out, e.Name())
	}
	return out, nil
}
