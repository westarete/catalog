package settings

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

var (
	defaultEnumerate = []string{"**/*.csv", "**/*.md", "**/*.pdf", "**/*.txt"}
	defaultIgnore    = []string{"README.md", "**/CLAUDE.md", "**/AGENTS.md"}
)

func TestParseGlobalDefaults(t *testing.T) {
	g, err := ParseGlobal([]byte(""))
	if err != nil {
		t.Fatal(err)
	}
	if g.Model != "claude-haiku-5-5" {
		t.Errorf("Model = %q", g.Model)
	}
	if !reflect.DeepEqual(g.Enumerate, defaultEnumerate) {
		t.Errorf("Enumerate = %v", g.Enumerate)
	}
	if !reflect.DeepEqual(g.Ignore, defaultIgnore) {
		t.Errorf("Ignore = %v", g.Ignore)
	}
}

func TestParseGlobalKeepsSetValues(t *testing.T) {
	g, err := ParseGlobal([]byte(`
api_key = "sk-test"
model = "other-model"
enumerate = ["a"]
ignore = []
`))
	if err != nil {
		t.Fatal(err)
	}
	if g.APIKey != "sk-test" || g.Model != "other-model" {
		t.Errorf("got %+v", g)
	}
	if !reflect.DeepEqual(g.Enumerate, []string{"a"}) {
		t.Errorf("Enumerate = %v", g.Enumerate)
	}
	if len(g.Ignore) != 0 {
		t.Errorf("explicit empty ignore was replaced: %v", g.Ignore)
	}
}

func TestParseGlobalWatch(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	g, err := ParseGlobal([]byte(`
[[watch]]
folder = "~/notes"

[[watch]]
folder = "/abs/path"
`))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{filepath.Join(home, "notes"), "/abs/path"}
	if !reflect.DeepEqual(g.Watch, want) {
		t.Errorf("Watch = %v, want %v", g.Watch, want)
	}
}

func TestParseGlobalMalformed(t *testing.T) {
	if _, err := ParseGlobal([]byte("model = ")); err == nil {
		t.Error("want error")
	}
}

func TestApplyCatalogReplacesLists(t *testing.T) {
	g, _ := ParseGlobal(nil)
	enumerate, ignore, err := ApplyCatalog(g, []byte(`enumerate = ["**/*.md"]`))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(enumerate, []string{"**/*.md"}) {
		t.Errorf("enumerate = %v", enumerate)
	}
	if !reflect.DeepEqual(ignore, g.Ignore) {
		t.Errorf("ignore = %v", ignore)
	}
}

func TestApplyCatalogReplacesIgnore(t *testing.T) {
	g, _ := ParseGlobal(nil)
	enumerate, ignore, err := ApplyCatalog(g, []byte(`ignore = []`))
	if err != nil {
		t.Fatal(err)
	}
	if len(ignore) != 0 {
		t.Errorf("ignore = %v", ignore)
	}
	if !reflect.DeepEqual(enumerate, g.Enumerate) {
		t.Errorf("enumerate = %v", enumerate)
	}
}

func TestApplyCatalogIgnoresOtherKeys(t *testing.T) {
	g, _ := ParseGlobal(nil)
	enumerate, ignore, err := ApplyCatalog(g, []byte(`
api_key = "sk-other"
model = "other"
profile = "profiles/default.md"
input_price = 1.5
output_price = 7.5
`))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(enumerate, g.Enumerate) || !reflect.DeepEqual(ignore, g.Ignore) {
		t.Errorf("lists changed: %v %v", enumerate, ignore)
	}
}

func TestApplyCatalogEmpty(t *testing.T) {
	g, _ := ParseGlobal(nil)
	for _, data := range []string{"", "# only a comment\n"} {
		enumerate, ignore, err := ApplyCatalog(g, []byte(data))
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(enumerate, g.Enumerate) || !reflect.DeepEqual(ignore, g.Ignore) {
			t.Errorf("lists changed for %q", data)
		}
	}
}

func TestApplyCatalogMalformed(t *testing.T) {
	g, _ := ParseGlobal(nil)
	enumerate, ignore, err := ApplyCatalog(g, []byte("enumerate = ["))
	if err == nil {
		t.Fatal("want error")
	}
	if enumerate != nil || ignore != nil {
		t.Errorf("want nil lists, got %v %v", enumerate, ignore)
	}
}

func TestStateDir(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	got, err := StateDir()
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(home, "Library", "Application Support", "com.westarete.catalog")
	if got != want {
		t.Errorf("StateDir = %q, want %q", got, want)
	}
}
