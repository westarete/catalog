package settings

import (
	"errors"
	"fmt"
	"regexp"

	"github.com/BurntSushi/toml"
)

// KeysFile is the name of the API keys file in StateDir.
const KeysFile = "keys.toml"

// DefaultKeyName is the key the daemon uses when global.toml names none.
const DefaultKeyName = "default"

// Keys maps each key's name to its API key.
type Keys map[string]string

// keysErrorLine matches the line the TOML decoder reports for a value of
// the wrong type. The rest of that message names the key, which a person
// may have pasted as text, so only the line is used.
var keysErrorLine = regexp.MustCompile(`line (\d+)`)

// ReadKeysFile reads and parses the keys.toml at path. It refuses the file
// for the same reasons, and with the same errors, as ReadGlobalFile.
func ReadKeysFile(path string) (Keys, error) {
	data, err := readPrivateFile(path)
	if err != nil {
		return nil, err
	}
	return ParseKeys(data)
}

// ParseKeys parses keys.toml. An error gives the line and never quotes a
// key or the name of one.
func ParseKeys(data []byte) (Keys, error) {
	keys := Keys{}
	if _, err := toml.Decode(string(data), &keys); err != nil {
		return nil, keysParseError(err)
	}
	return keys, nil
}

func keysParseError(err error) error {
	if pe, ok := errors.AsType[toml.ParseError](err); ok {
		return fmt.Errorf("parse %s: line %d: not valid TOML", KeysFile, pe.Position.Line)
	}
	if m := keysErrorLine.FindStringSubmatch(err.Error()); m != nil {
		return fmt.Errorf("parse %s: line %s: a key's value isn't a string", KeysFile, m[1])
	}
	return fmt.Errorf("parse %s: not valid TOML", KeysFile)
}
