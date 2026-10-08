package pagination

import "testing"

func TestNewClampsEveryInput(t *testing.T) {
	for _, c := range []struct {
		page, size, max, wantPage, wantSize int
	}{
		{0, 0, 50, DefaultPage, DefaultPageSize},
		{3, 500, 50, 3, 50},
		{1, 500, 0, 1, AbsoluteMaxPageSize},      // teto padrão
		{1, 500, 100000, 1, AbsoluteMaxPageSize}, // teto acima do absoluto
	} {
		p := New(c.page, c.size, c.max)
		if p.Page != c.wantPage || p.PageSize != c.wantSize {
			t.Errorf("New(%d,%d,%d) = %+v", c.page, c.size, c.max, p)
		}
	}
	p := New(3, 20, 100)
	if p.Offset() != 40 || p.Limit() != 20 {
		t.Fatalf("offset/limit: %d %d", p.Offset(), p.Limit())
	}
	m := NewMeta(p, 41)
	if m.TotalPages != 3 || m.TotalItems != 41 {
		t.Fatalf("meta: %+v", m)
	}
	if NewMeta(Params{}, 10).TotalPages != 0 {
		t.Fatal("page_size zero não divide por zero")
	}
}
