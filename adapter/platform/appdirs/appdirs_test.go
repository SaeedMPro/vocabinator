package appdirs

import (
	"path/filepath"
	"testing"
)

func TestDataDir(t *testing.T) {
	home := func() (string, error) { return filepath.FromSlash("/home/u"), nil }
	env := func(m map[string]string) func(string) string { return func(k string) string { return m[k] } }

	cases := []struct {
		name string
		goos string
		env  map[string]string
		want string
		fail bool
	}{
		{"linux default", "linux", nil, "/home/u/.local/share/vocabinator", false},
		{"linux xdg", "linux", map[string]string{"XDG_DATA_HOME": "/data"}, "/data/vocabinator", false},
		{"linux relative xdg ignored", "linux", map[string]string{"XDG_DATA_HOME": "rel"}, "/home/u/.local/share/vocabinator", false},
		{"darwin", "darwin", nil, "/home/u/Library/Application Support/vocabinator", false},
		{"windows", "windows", map[string]string{"LocalAppData": "/appdata"}, "/appdata/vocabinator", false},
		{"windows unset", "windows", nil, "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := dataDir(tc.goos, env(tc.env), home)
			if (err != nil) != tc.fail {
				t.Fatalf("unexpected error state: %v", err)
			}
			if want := filepath.FromSlash(tc.want); got != want {
				t.Errorf("want %q, got %q", want, got)
			}
		})
	}
}
