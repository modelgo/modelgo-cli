package version

import "testing"

func TestDefaultIsDev(t *testing.T) {
	if Version != "dev" {
		t.Fatalf("expected default Version to be %q, got %q", "dev", Version)
	}
}
