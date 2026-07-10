package provision

import "strings"

import "testing"

func TestClassify(t *testing.T) {
	cases := []struct {
		name string
		r    Reachability
		want NetworkMode
	}{
		{"ambos ok", Reachability{PublicOK: true, TailnetOK: true}, ModeClean},
		{"solo tailnet (excluido del tunel)", Reachability{PublicOK: false, TailnetOK: true}, ModeSplitExcluded},
		{"solo publico (atrapado en el tunel)", Reachability{PublicOK: true, TailnetOK: false}, ModeTunneled},
		{"ninguno", Reachability{PublicOK: false, TailnetOK: false}, ModeIsolated},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := Classify(c.r); got != c.want {
				t.Fatalf("Classify(%+v) = %v; quería %v", c.r, got, c.want)
			}
		})
	}
}

func TestModeOK(t *testing.T) {
	// Clean y SplitExcluded permiten seguir hasta el self-check; los otros frenan.
	if !ModeClean.ok() || !ModeSplitExcluded.ok() {
		t.Fatal("Clean y SplitExcluded deberían permitir continuar")
	}
	if ModeTunneled.ok() || ModeIsolated.ok() {
		t.Fatal("Tunneled e Isolated no deberían continuar")
	}
}

func TestGuidanceIsVPNAgnostic(t *testing.T) {
	// Ningún modo debe nombrar un producto de VPN concreto (requisito R2).
	for _, m := range []NetworkMode{ModeClean, ModeSplitExcluded, ModeTunneled, ModeIsolated} {
		g := Guidance(m, "brain.example:7717")
		if g == "" {
			t.Fatalf("Guidance(%v) vacía", m)
		}
		if strings.Contains(strings.ToLower(g), "nord") {
			t.Errorf("Guidance(%v) nombra un producto de VPN concreto: %q", m, g)
		}
	}
}

func TestGuidanceTunneledMentionsTailnetRange(t *testing.T) {
	// El modo Tunneled (el riesgo en otra PC) debe dar el paso accionable: abrir el rango.
	g := Guidance(ModeTunneled, "1.2.3.4:7717")
	if !strings.Contains(g, tailnetCIDR) {
		t.Errorf("Guidance(Tunneled) debería citar el rango del tailnet %s; got %q", tailnetCIDR, g)
	}
	if !strings.Contains(g, "1.2.3.4:7717") {
		t.Errorf("Guidance(Tunneled) debería citar el brain; got %q", g)
	}
}

func TestPreflightUsesProber(t *testing.T) {
	p := &fakeProber{public: false, tailnet: true}
	r, m := Preflight(p, "brain:7717")
	if !r.TailnetOK || r.PublicOK {
		t.Fatalf("Reachability inesperada: %+v", r)
	}
	if m != ModeSplitExcluded {
		t.Fatalf("modo = %v; quería SplitExcluded", m)
	}
}
