// Package skills carga y resuelve las skills (reglas de comportamiento en YAML)
// que Musubi inyecta según el contexto del proyecto.
package skills

import "sort"

// Skill representa una regla de comportamiento cargada desde un archivo YAML.
type Skill struct {
	Name         string   `yaml:"name"`
	Description  string   `yaml:"description"`
	Triggers     []string `yaml:"triggers"`
	Capabilities []string `yaml:"capabilities"`
	Rules        string   `yaml:"rules"`
	// AppliesTo declara el alcance de la skill por FASE DEL TRABAJO o FORMA DE LA TAREA, para las
	// que no se activan por archivo. Vocabulario CERRADO: ver VocabularioDeAlcance.
	//
	// NO ES UNA TAXONOMÍA INVENTADA: está transcrita de los `always_because` que los autores ya
	// habían escrito. Seis skills declararon con sus palabras qué eje necesitaban —«planificar es
	// una FASE», «se activa por la FORMA de la tarea», «el disparador es el momento»— y caen en
	// dos ejes. El campo le da al resolvedor lo que hasta ahora sólo podía leer un humano.
	//
	// Es ADITIVO: una skill puede tener AppliesTo y Triggers a la vez, y matchea por cualquiera de
	// los dos. omitempty: una skill que se activa por archivo no lo necesita.
	AppliesTo []string `yaml:"applies_to,omitempty"`
	// GeneratedBy indica el origen de la skill; "auto-discovery" para skills generadas automáticamente.
	// omitempty mantiene el YAML limpio para skills escritas a mano.
	GeneratedBy string `yaml:"generated_by,omitempty"`
	// GeneratedAt es la marca de tiempo RFC3339 en que se generó la skill; vacío para skills a mano.
	GeneratedAt string `yaml:"generated_at,omitempty"`
	// Source identifica el catálogo o repositorio de origen de la skill (p.ej. "musubi-catalog-v1").
	// omitempty mantiene el YAML limpio para skills escritas a mano sin procedencia de catálogo.
	Source string `yaml:"source,omitempty"`
	// SourceURL es la URL al archivo de reglas completo en el catálogo de origen.
	// omitempty mantiene el YAML limpio cuando el campo está vacío.
	SourceURL string `yaml:"source_url,omitempty"`
	// AlwaysBecause declara POR QUÉ esta skill se activa siempre (trigger "*").
	//
	// Existe porque el vocabulario de triggers sólo sabe de globs de ARCHIVO, y hay skills que no
	// se activan por archivo sino por TIPO DE TAREA: orquestar, planificar, auditar. Ésas no tienen
	// cómo expresarse salvo con "*". Medido sobre las 11 skills de este repo, 6 usan "*" y ninguna
	// por descuido. Sin este campo, un "*" deliberado y uno por pereza son indistinguibles —y en un
	// arsenal compartido esa diferencia decide si el contexto de otro se llena de reglas que no
	// aplican. omitempty: una skill con triggers acotados no lo necesita.
	//
	// RESULTÓ SER MÁS QUE UNA JUSTIFICACIÓN. Al leer los seis que existen se vio que cada autor
	// había escrito, en prosa, exactamente el eje que le faltaba al vocabulario. De ahí sale
	// AppliesTo: este campo fue su corpus. Sigue siendo para humanos; AppliesTo es su forma
	// matcheable, y conviven.
	AlwaysBecause string `yaml:"always_because,omitempty"`
	// ManagedChecksum es el sha256 (hex) del contenido canónico de una skill COGNITIVA
	// manejada por Musubi (writeCognitiveSkills), con este mismo campo vacío. Es la prueba de
	// si el archivo sigue tal como Musubi lo escribió: si el checksum del archivo coincide, la
	// skill está intacta y una actualización del binario la refresca; si no coincide (o no
	// existe), fue editada a mano y se preserva. Vacío/omitempty para skills a mano o de
	// auto-discovery (no las gestiona Musubi).
	ManagedChecksum string `yaml:"managed_checksum,omitempty"`
}

// EL VOCABULARIO DE ALCANCE, v1. Cerrado a propósito, como validOutcome del ledger o el enum de
// predicados de cognición: con texto libre, un typo (`phase:planing`) se vería igual que una skill
// que simplemente nunca aplica — silencioso, y el peor modo de falla posible.
//
// Nace con CINCO valores porque son los cinco que alguien escribió. Cada uno tiene un autor real
// detrás, no un hueco imaginado; crece con evidencia y no con imaginación.
const (
	// FasePlanificar — de plan-ahead: «planificar es una FASE del trabajo, no un tipo de archivo».
	FasePlanificar = "phase:planning"
	// FaseImplementar — de sdd-flow: «gobierna el flujo completo de un cambio».
	FaseImplementar = "phase:implementing"
	// FaseRevisar — de adversarial-review: «se activa al cerrar un cambio de riesgo; el disparador
	// es el momento, no el archivo».
	FaseRevisar = "phase:reviewing"
	// TareaAuditar — de audit-structure-flow: «el disparador es el pedido de auditoría».
	TareaAuditar = "task:audit"
	// TareaOrquestar — de orchestrate-multiagent: «se activa por la FORMA de la tarea (grande y
	// paralelizable)».
	TareaOrquestar = "task:orchestration"
)

var vocabularioAlcance = map[string]bool{
	FasePlanificar:  true,
	FaseImplementar: true,
	FaseRevisar:     true,
	TareaAuditar:    true,
	TareaOrquestar:  true,
}

// VocabularioDeAlcance devuelve los valores válidos de AppliesTo, ordenados. Para el validador y
// para que un mensaje de error pueda decir qué SÍ se acepta en vez de sólo qué no.
func VocabularioDeAlcance() []string {
	out := make([]string, 0, len(vocabularioAlcance))
	for v := range vocabularioAlcance {
		out = append(out, v)
	}
	sort.Strings(out)
	return out
}

// AlcanceValido indica si v pertenece al vocabulario cerrado.
func AlcanceValido(v string) bool { return vocabularioAlcance[v] }

// ResolveRequest es lo que el llamador sabe de su situación. Reemplaza al `[]string` de archivos
// porque el problema de fondo NO era el vocabulario de triggers sino la PREGUNTA: mientras la única
// entrada fueran archivos, una skill que no habla de archivos no tenía cómo ser encontrada.
//
// Phase y Task son DECLARADOS por el llamador, no inferidos. Es lo que mantiene el matcher
// determinista y gratis: el agente ya sabe en qué fase está, sólo le faltaba dónde decirlo. Vacíos
// ⇒ no se declaró nada y sólo matchean los globs.
type ResolveRequest struct {
	ModifiedFiles []string
	Phase         string
	Task          string
}

// DeclaraAlgo indica si el llamador dijo ALGO — archivos o alcance. Una petición que no dice nada
// no tiene respuesta útil: devolver el arsenal entero sería peor que fallar.
func (r ResolveRequest) DeclaraAlgo() bool {
	return len(r.ModifiedFiles) > 0 || r.Phase != "" || r.Task != ""
}

type Resolver struct {
	skillsDir string
}

func NewResolver(projectPath string) *Resolver {
	return &Resolver{
		skillsDir: projectPath,
	}
}
