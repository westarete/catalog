package settings

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
)

func TestParseKeysReadsNamedKeys(t *testing.T) {
	got, err := ParseKeys([]byte("default = \"sk-a\"\nacme = \"sk-b\"\n"))
	want := Keys{"default": "sk-a", "acme": "sk-b"}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Errorf("ParseKeys = %v, %v; want %v", got, err, want)
	}
}

func TestParseKeysCommentOnly(t *testing.T) {
	got, err := ParseKeys([]byte("# no keys yet\n\n# add one here\n"))
	if err != nil || got == nil || len(got) != 0 {
		t.Errorf("ParseKeys = %#v, %v; want an empty map and no error", got, err)
	}
}

func TestParseKeysErrorsQuoteNothing(t *testing.T) {
	line := regexp.MustCompile(`line \d+`)
	for _, text := range []string{
		"sk-secret-1",
		"default = \"x\"\nsk-secret-2 = 3",
		"default = sk-secret-3",
	} {
		_, err := ParseKeys([]byte(text))
		if err == nil {
			t.Errorf("%q: err = nil; want an error", text)
			continue
		}
		msg := err.Error()
		if !line.MatchString(msg) || strings.Contains(msg, "sk-secret") {
			t.Errorf("%q: error %q; want a line number and no key text", text, msg)
		}
	}
}

func TestParseKeysNonStringValueNamesLine(t *testing.T) {
	_, err := ParseKeys([]byte("default = \"x\"\n\nacme = 3\n"))
	if err == nil || !strings.Contains(err.Error(), "line 3") {
		t.Errorf("err = %v; want one naming line 3", err)
	}
}

func TestReadKeysFileReadsKeys(t *testing.T) {
	path := writeWithMode(t, `default = "sk-a"`, 0o600)
	got, err := ReadKeysFile(path)
	if err != nil || got[DefaultKeyName] != "sk-a" {
		t.Errorf("ReadKeysFile = %v, %v; want the default key", got, err)
	}
}

func TestReadKeysFileRefusesWorldAccessible(t *testing.T) {
	path := writeWithMode(t, `default = "sk-secret"`, 0o644)
	got, err := ReadKeysFile(path)
	if !errors.Is(err, ErrWorldAccessible) || got != nil {
		t.Errorf("ReadKeysFile = %v, %v; want no keys and ErrWorldAccessible", got, err)
	}
}

func TestReadKeysFileRefusesNonRegular(t *testing.T) {
	path := filepath.Join(t.TempDir(), KeysFile)
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadKeysFile(path); !errors.Is(err, ErrNotRegularFile) {
		t.Errorf("err = %v; want ErrNotRegularFile", err)
	}
}
