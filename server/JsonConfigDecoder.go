package server

import (
	"encoding/json"
	"github.com/erocheleau/uabot/explorerlib"
	"io"
)

func DecodeConfig(jsonReader io.Reader) (*explorerlib.Config, error) {
	config := &explorerlib.Config{}
	err := json.NewDecoder(jsonReader).Decode(config)
	return config, err
}
