package repovivo

import (
	"context"
	"fmt"
	"strings"
	"sync"
)

// prefijoRefVivo es el único espacio de refs donde la zona en vivo publica. Vive FUERA de
// refs/heads/ a propósito: el espejo de Altura replica refs/heads/* y refs/tags/* a GitHub,
// así que una foto en refs/heads/ se filtraría al repo público. El constructor del transporte
// rechaza cualquier ref que no empiece con esto: es una guarda estructural, no una convención.
const prefijoRefVivo = "refs/vivo/"

// transporte publica una foto (un commit huérfano) en la ref de este checkout, en el remoto
// empresarial, sin pisar a ciegas un avance ajeno. Es de UN checkout: su Mutex es el candado
// "un solo subidor por checkout", y ultimoSha es lo último que ESTE transporte subió con éxito,
// que sirve de lease para el --force-with-lease del próximo push.
type transporte struct {
	git    gitRunner
	remoto string
	ref    string

	mu        sync.Mutex
	ultimoSha string
}

// nuevoTransporte valida los valores que vienen de afuera y arma el transporte. El remoto
// (de la config del usuario) y la ref no pueden parecer flags, el remoto no puede ser vacío, y
// la ref DEBE estar bajo refs/vivo/ (seguridad del espejo). Falla temprano si algo no cumple.
func nuevoTransporte(git gitRunner, remoto, ref string) (*transporte, error) {
	if err := rechazarFlags(remoto, ref); err != nil {
		return nil, err
	}
	if strings.TrimSpace(remoto) == "" {
		return nil, fmt.Errorf("remoto vacío")
	}
	if !strings.HasPrefix(ref, prefijoRefVivo) {
		return nil, fmt.Errorf("ref %q fuera de %s: la zona en vivo no publica en otro lado", ref, prefijoRefVivo)
	}
	return &transporte{git: git, remoto: remoto, ref: ref}, nil
}

// publicar sube la foto `sha` a la ref de este checkout. Secuencia:
//  1. intento con el lease de lo último que subimos (vacío la primera vez → push de creación);
//  2. si lo rechazan, leemos el sha REAL de la ref en el remoto (ls-remote);
//     - si no podemos ni leer el remoto, es la red: devolvemos el error, sin reintentar;
//     - si la ref remota YA es nuestra foto, damos por hecho el push;
//  3. reintentamos UNA vez con el lease del sha real.
//
// Pisar tras enterarnos del sha real es correcto acá: la ref es de ESTE nodo+checkout y las
// fotos se pisan por diseño; el lease sólo evita pisar SIN enterarnos. El Mutex serializa los
// push del mismo checkout.
func (t *transporte) publicar(ctx context.Context, sha string) error {
	if err := rechazarFlags(sha); err != nil {
		return err
	}
	t.mu.Lock()
	defer t.mu.Unlock()

	refspec := sha + ":" + t.ref
	err := t.pushConLease(ctx, refspec, t.ultimoSha)
	if err == nil {
		t.ultimoSha = sha
		return nil
	}

	shaRemoto, lsErr := t.shaEnRemoto(ctx)
	if lsErr != nil {
		return fmt.Errorf("push falló y no pude leer el remoto (¿red?): %w", err)
	}
	if shaRemoto == sha {
		t.ultimoSha = sha
		return nil
	}
	if err2 := t.pushConLease(ctx, refspec, shaRemoto); err2 != nil {
		return fmt.Errorf("push rechazado también en el reintento con el lease real: %w", err2)
	}
	t.ultimoSha = sha
	return nil
}

// pushConLease empuja refspec al remoto. Con expect no vacío agrega --force-with-lease=<ref>:<expect>,
// que permite pisar (incluso con un commit huérfano, no fast-forward) SÓLO si la ref remota está
// en <expect>; si cambió, git rechaza. Con expect vacío hace un push de creación normal (la ref
// aún no existe para nosotros): si ya existiera y no fuera fast-forward, git lo rechaza y publicar
// pasa a enterarse del sha real.
//
// --no-verify es obligatorio (R3): el push NO DEBE disparar el hook pre-push del repo del usuario
// (p. ej. el de migraciones de Altura). La zona en vivo es un respaldo de fondo que no debe gatillar
// los controles que el autor corre a mano; sin esto, un pre-push que falla abortaría la foto.
func (t *transporte) pushConLease(ctx context.Context, refspec, expect string) error {
	args := []string{"push", "--porcelain", "--no-verify"}
	if expect != "" {
		args = append(args, "--force-with-lease="+t.ref+":"+expect)
	}
	args = append(args, t.remoto, refspec)
	_, err := t.git.run(ctx, nil, args...)
	return err
}

// shaEnRemoto lee el sha de la ref en el remoto con ls-remote (más liviano que un fetch). Devuelve
// "" sin error si la ref todavía no existe allá. Un error significa que no se pudo hablar con el
// remoto (red/credencial), no que la ref no exista.
func (t *transporte) shaEnRemoto(ctx context.Context) (string, error) {
	out, err := t.git.run(ctx, nil, "ls-remote", t.remoto, t.ref)
	if err != nil {
		return "", err
	}
	campos := strings.Fields(out) // "<sha>\t<ref>"
	if len(campos) == 0 {
		return "", nil
	}
	return campos[0], nil
}
