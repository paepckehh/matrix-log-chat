package version

import "testing"

func TestString(t *testing.T) {
	t.Run("default", func(t *testing.T) {
		saved := Version
		Build = ""
		Version = "v0.0.0-dev"
		if got := String(); got != "v0.0.0-dev" {
			t.Fatalf("got %q, want %q", got, "v0.0.0-dev")
		}
		Version = saved
	})
	t.Run("with build metadata", func(t *testing.T) {
		savedV, savedB := Version, Build
		Version = "v1.2.3"
		Build = "abc123"
		if got := String(); got != "v1.2.3 (abc123)" {
			t.Fatalf("got %q, want %q", got, "v1.2.3 (abc123)")
		}
		Version, Build = savedV, savedB
	})
}
