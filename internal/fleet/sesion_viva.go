package fleet

// sesion_viva.go es la VISTA COMÚN del plano de entrar: una sola forma para preguntar «quién está
// adentro de mis máquinas, y desde cuándo».
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// POR QUÉ ES UNA VISTA Y NO UNA TABLA ÚNICA — Y POR QUÉ LA PRIMERA INTENCIÓN ERA LA OTRA
//
// Las dos tablas de sesión (`screen_sessions` y `shell_sessions`) son casi idénticas: mismas
// columnas salvo una. Eso invita a fusionarlas, y la maqueta de tres planos lo proponía. Al ir a
// hacerlo, la encuesta del código dijo otra cosa: **las TABLAS se parecen, los COMPORTAMIENTOS
// no.**
//
// Una shell tiene `UltimoTrafico` —que alimenta un techo de inactividad—, una sola sesión abierta
// por (principal × máquina), y un barrendero que cierra las vencidas. Una sesión de pantalla no
// tiene ninguna de las tres: se acuña una contraseña, se entrega, y vence sola. Fusionarlas daría
// una tabla con columnas que sólo aplican a la mitad de sus filas, que es exactamente el olor de
// esquema que este repo evita en todos lados.
//
// Así que lo que se comparte es la FORMA DE AUDITORÍA —quién, dónde, cuándo, cómo terminó— y eso
// es una vista. La consola necesita listar; no necesita que sean la misma fila.
//
// La decisión inversa —fusionar igual— se puede tomar el día que aparezca una tercera modalidad
// que se comporte como una de las dos. Hoy no existe.

import "time"

// Modalidad es POR DÓNDE entró alguien. No es un detalle de presentación: cada una tiene techos y
// riesgos distintos, y una lista que no las distingue no sirve para decidir nada.
type Modalidad string

const (
	ModalidadPantalla Modalidad = "pantalla"
	ModalidadShell    Modalidad = "shell"
)

// SesionViva es la forma común. Deliberadamente NO tiene los campos propios de cada modalidad:
// meter `UltimoTrafico` acá haría que la mitad de las filas lo tengan en cero y que ese cero se
// lea como «sin tráfico» en vez de como «no aplica» — el mismo error que el resto del dominio
// persigue.
type SesionViva struct {
	ID        string
	Modalidad Modalidad
	DeviceID  string
	// Device es el NOMBRE legible. Va acá y no se deja resolver al llamador porque una lista de
	// sesiones con ids opacos no la lee nadie.
	Device    string
	ProjectID string
	Principal string
	// Estado viaja como texto porque cada modalidad tiene su propio enum y no son el mismo
	// conjunto. Traducirlos a uno común inventaría estados que ninguna de las dos tiene.
	Estado  string
	Creada  time.Time
	Vence   time.Time
	Cerrada time.Time
	Error   string
}

// Abierta dice si la sesión sigue en curso a esta hora.
//
// SE DERIVA, no se guarda. Es la misma regla que el «en línea» de un dispositivo y que `Vencida`:
// una columna de estado que hay que ir a actualizar miente en cuanto nadie la actualiza — y acá
// mentiría diciendo que alguien sigue adentro de una máquina cuando ya salió, que es la mentira
// más cara que puede decir un panel de este plano.
func (s SesionViva) Abierta(ahora time.Time) bool {
	return s.Cerrada.IsZero() && !s.Vence.IsZero() && ahora.Before(s.Vence)
}

// DesdeSesionPantalla y DesdeSesionShell son las dos únicas puertas a la vista común. Existen
// para que la traducción viva en el dominio y no en cada consumidor: dos consumidores que
// traducen por su cuenta terminan discrepando en qué es «abierta».
func DesdeSesionPantalla(s SesionPantalla, device string) SesionViva {
	return SesionViva{
		ID: s.ID, Modalidad: ModalidadPantalla, DeviceID: s.DeviceID, Device: device,
		ProjectID: s.ProjectID, Principal: s.Principal, Estado: string(s.Estado),
		Creada: s.Creada, Vence: s.Vence, Cerrada: s.Cerrada, Error: s.Error,
	}
}

func DesdeSesionShell(s SesionShell, device string) SesionViva {
	return SesionViva{
		ID: s.ID, Modalidad: ModalidadShell, DeviceID: s.DeviceID, Device: device,
		ProjectID: s.ProjectID, Principal: s.Principal, Estado: string(s.Estado),
		Creada: s.Creada, Vence: s.Vence, Cerrada: s.Cerrada, Error: s.Error,
	}
}
