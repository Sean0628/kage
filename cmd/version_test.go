package cmd

import "testing"

func TestVersionStringUsesInjectedVersion(t *testing.T) {
	original := version
	version = "v1.2.3"
	t.Cleanup(func() {
		version = original
	})

	if got := versionString(); got != "v1.2.3" {
		t.Fatalf("versionString() = %q, want %q", got, "v1.2.3")
	}
}

func TestVersionStringFallsBackToDev(t *testing.T) {
	original := version
	version = "dev"
	t.Cleanup(func() {
		version = original
	})

	got := versionString()
	if got == "" {
		t.Fatal("versionString() returned an empty string")
	}
}
