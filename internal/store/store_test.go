package store

import "testing"

func TestOptionsHashDeterministic(t *testing.T) {
	a := OptionsHash("example.com", true, false)
	b := OptionsHash("example.com", true, false)
	if a != b {
		t.Errorf("hash not deterministic: %s != %s", a, b)
	}
}

func TestOptionsHashVariesByInput(t *testing.T) {
	base := OptionsHash("example.com", false, false)
	cases := []string{
		OptionsHash("example.org", false, false),
		OptionsHash("example.com", true, false),
		OptionsHash("example.com", false, true),
	}
	for i, h := range cases {
		if h == base {
			t.Errorf("case %d produced same hash as base", i)
		}
	}
}

func TestNormalizeDSN(t *testing.T) {
	cases := map[string]string{
		"postgres://u:p@h:5432/db":   "pgx5://u:p@h:5432/db",
		"postgresql://u:p@h:5432/db": "pgx5://u:p@h:5432/db",
		"pgx5://u:p@h:5432/db":       "pgx5://u:p@h:5432/db",
	}
	for in, want := range cases {
		if got := normalizeDSN(in); got != want {
			t.Errorf("normalizeDSN(%q) = %q, want %q", in, got, want)
		}
	}
}
