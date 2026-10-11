package main

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

// inEmptyDir runs the test in a fresh directory with no .catalog/ yet, which
// newTempRepo does not give us.
func inEmptyDir(t *testing.T) {
	t.Helper()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(orig); err != nil {
			t.Errorf("cleanup: restore working directory: %v", err)
		}
	})
}

func TestInitCreatesTheConfigFromTheTemplate(t *testing.T) {
	inEmptyDir(t)
	var out bytes.Buffer
	if err := runInit(nil, &out); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != exampleConfig {
		t.Error("config.toml does not match the embedded template")
	}
	want := "created .catalog/config.toml\nedit enumerate and ignore for this repo, then run `catalog bootstrap`\n"
	if out.String() != want {
		t.Errorf("output %q, want %q", out.String(), want)
	}
}

func TestInitRefusesToOverwriteAnExistingConfig(t *testing.T) {
	inEmptyDir(t)
	writeFile(t, configPath, "model = \"mine\"\n")
	err := runInit(nil, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "already exists") || !strings.Contains(err.Error(), "--print") {
		t.Fatalf("error = %v, want one saying it exists and pointing at --print", err)
	}
	if got, _ := os.ReadFile(configPath); string(got) != "model = \"mine\"\n" {
		t.Errorf("existing config was changed: %q", got)
	}
}

func TestInitPrintWritesTheTemplateAndTouchesNothing(t *testing.T) {
	inEmptyDir(t)
	var out bytes.Buffer
	if err := runInit([]string{"--print"}, &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != exampleConfig {
		t.Error("--print did not write the embedded template")
	}
	if _, err := os.Stat(".catalog"); !os.IsNotExist(err) {
		t.Errorf("--print created .catalog (stat error: %v)", err)
	}
}

func TestInitPrintWorksWhereAConfigAlreadyExists(t *testing.T) {
	inEmptyDir(t)
	writeFile(t, configPath, "model = \"mine\"\n")
	var out bytes.Buffer
	if err := runInit([]string{"--print"}, &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != exampleConfig {
		t.Error("--print should show the template even when a config exists")
	}
}

func TestInitRejectsUnknownArguments(t *testing.T) {
	inEmptyDir(t)
	for _, tc := range []struct {
		args []string
		bad  string // the argument the error should name
	}{
		{[]string{"--force"}, "--force"},
		{[]string{"--print", "extra"}, "extra"},
		{[]string{"somewhere"}, "somewhere"},
	} {
		err := runInit(tc.args, &bytes.Buffer{})
		if err == nil || !strings.Contains(err.Error(), `"`+tc.bad+`"`) {
			t.Errorf("runInit(%v) = %v, want an error naming %q", tc.args, err, tc.bad)
		}
	}
	if _, err := os.Stat(configPath); !os.IsNotExist(err) {
		t.Error("a rejected init still wrote a config")
	}
}

func TestExampleConfigParses(t *testing.T) {
	cfg, err := parseConfig([]byte(exampleConfig))
	if err != nil {
		t.Fatalf("the embedded template does not parse: %v", err)
	}
	if cfg.Model != defaultModel {
		t.Errorf("template model %q differs from defaultModel %q; keep them in step", cfg.Model, defaultModel)
	}
	if len(cfg.Enumerate) == 0 || len(cfg.Ignore) == 0 {
		t.Errorf("template should show enumerate and ignore, got %v and %v", cfg.Enumerate, cfg.Ignore)
	}
	if cfg.prices() == nil || cfg.WarnRequestTokens == 0 {
		t.Errorf("template should show prices and warn_request_tokens, got %+v and %d", cfg.prices(), cfg.WarnRequestTokens)
	}
	if len(cfg.warnings) != 0 {
		t.Errorf("template has keys the config does not recognize: %v", cfg.warnings)
	}
}

func TestMissingConfigErrorPointsAtInit(t *testing.T) {
	inEmptyDir(t)
	_, err := loadConfig()
	if err == nil || !strings.Contains(err.Error(), "catalog init") {
		t.Errorf("error = %v, want one that points at `catalog init`", err)
	}
}
