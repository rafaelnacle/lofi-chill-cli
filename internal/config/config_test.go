package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRoundTripAndMigration(t *testing.T) {
	p := filepath.Join(t.TempDir(), "nested", "config.json")
	c := Default()
	c.Volumes["rain"] = 70
	c.Chime = false
	c.Completed = 3
	if e := Save(p, c); e != nil {
		t.Fatal(e)
	}
	got, e := Load(p)
	if e != nil || got.Volumes["rain"] != 70 || got.Chime || got.Completed != 3 {
		t.Fatal(got, e)
	}
	if e = os.WriteFile(p, []byte(`{"volumes":{"brown":42},"durations":[0,121,9]}`), 0600); e != nil {
		t.Fatal(e)
	}
	got, e = Load(p)
	if e != nil || got.Volumes["rain"] != 0 || got.Volumes["brown"] != 42 || got.Durations != [3]int{25, 5, 9} {
		t.Fatal(got, e)
	}
}
func TestBadConfig(t *testing.T) {
	p := filepath.Join(t.TempDir(), "config.json")
	for _, s := range []string{`{`, `{"version":99}`, `{"volumes":{"rain":"loud"}}`} {
		os.WriteFile(p, []byte(s), 0600)
		c, e := Load(p)
		if e == nil || c.Volumes["rain"] != 35 {
			t.Fatal(c, e)
		}
	}
}
