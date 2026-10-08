package domain

import "testing"

func TestCanTransition(t *testing.T) {
	allowed := map[[2]string]bool{
		{StatusDraft, StatusPublished}: true, {StatusDraft, StatusArchived}: true,
		{StatusPublished, StatusArchived}: true, {StatusPublished, StatusDraft}: true,
		{StatusArchived, StatusDraft}: true, {StatusArchived, StatusPublished}: true,
	}
	states := []string{StatusDraft, StatusPublished, StatusArchived, "desconhecido"}
	for _, from := range states {
		for _, to := range states {
			if got := CanTransition(from, to); got != allowed[[2]string{from, to}] {
				t.Errorf("%s -> %s: %v", from, to, got)
			}
		}
	}
}
