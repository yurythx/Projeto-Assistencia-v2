package audit

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestParseWindow(t *testing.T) {
	t.Run("vazio → últimos 30 dias", func(t *testing.T) {
		from, to, err := parseWindow("", "")
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		d := to.Sub(from)
		if d < 29*24*time.Hour || d > 31*24*time.Hour {
			t.Errorf("janela = %v, esperava ~30 dias", d)
		}
	})

	t.Run("AAAA-MM-DD", func(t *testing.T) {
		from, to, err := parseWindow("2026-01-01", "2026-02-01")
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		if from.Format("2006-01-02") != "2026-01-01" || to.Format("2006-01-02") != "2026-02-01" {
			t.Errorf("from=%v to=%v", from, to)
		}
	})

	t.Run("invertida → erro", func(t *testing.T) {
		if _, _, err := parseWindow("2026-02-01", "2026-01-01"); err == nil {
			t.Error("esperava erro para janela invertida")
		}
	})

	t.Run("maior que 366 dias → erro", func(t *testing.T) {
		if _, _, err := parseWindow("2024-01-01", "2026-01-01"); err == nil {
			t.Error("esperava erro para janela > 366 dias")
		}
	})

	t.Run("data inválida → erro", func(t *testing.T) {
		if _, _, err := parseWindow("ontem", ""); err == nil {
			t.Error("esperava erro para 'from' não-parseável")
		}
	})
}

func TestChooseFormat(t *testing.T) {
	cases := []struct{ query, accept, want string }{
		{"", "", "csv"},
		{"", "application/json", "json"},
		{"", "application/xml", "xml"},
		{"format=json", "text/csv", "json"}, // query vence o Accept
		{"format=xml", "application/json", "xml"},
		{"format=csv", "application/json", "csv"},
	}
	for _, c := range cases {
		r := httptest.NewRequest("GET", "/audit/export?"+c.query, nil)
		if c.accept != "" {
			r.Header.Set("Accept", c.accept)
		}
		if got := chooseFormat(r); got != c.want {
			t.Errorf("query=%q accept=%q → %q, want %q", c.query, c.accept, got, c.want)
		}
	}
}

func TestCursorRoundTrip(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Nanosecond)
	id := uuid.NewString()
	enc := encodeCursor(now, id)

	gotT, gotID, err := parseCursor(enc)
	if err != nil {
		t.Fatalf("parseCursor: %v", err)
	}
	if !gotT.Equal(now) {
		t.Errorf("time = %v, want %v", gotT, now)
	}
	if gotID != id {
		t.Errorf("id = %q, want %q", gotID, id)
	}

	if _, _, err := parseCursor("lixo"); err == nil {
		t.Error("esperava erro para cursor malformado")
	}
	if tt, _, err := parseCursor(""); err != nil || !tt.IsZero() {
		t.Errorf("cursor vazio deve ser (zero, nil), veio (%v, %v)", tt, err)
	}
}

func TestParseWindowAndCursorEdgeCases(t *testing.T) {
	if _, _, err := parseWindow("", "amanhã"); err == nil {
		t.Error("'to' inválido deveria falhar")
	}
	from, to, err := parseWindow("", "2026-03-31T12:00:00Z")
	if err != nil || to.Sub(from) != defaultExportWindow {
		t.Errorf("só 'to': janela padrão terminando em 'to' (%v→%v, %v)", from, to, err)
	}
	for _, c := range []string{"nao-e-data," + uuid.NewString(), "2026-01-01T00:00:00Z,nao-uuid"} {
		if _, _, err := parseCursor(c); err == nil {
			t.Errorf("cursor %q deveria ser recusado", c)
		}
	}
}
