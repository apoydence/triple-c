package main

import (
	"encoding/json"

	"code.cloudfoundry.org/go-envstruct"
)

type Config struct {
	// HttpProxy is not used directly, however the CAPI client assumes its
	// going through a proxy for auth.
	HttpProxy       string          `env:"HTTP_PROXY, required"`
	VcapApplication VcapApplication `env:"VCAP_APPLICATION, required"`
}

type VcapApplication struct {
	CAPIAddr        string   `json:"cf_api"`
	ApplicationID   string   `json:"application_id"`
	ApplicationURIs []string `json:"application_uris"`
	SpaceID         string   `json:"space_id"`
}

func (a *VcapApplication) UnmarshalEnv(data string) error {
	return json.Unmarshal([]byte(data), a)
}

func LoadConfig() (Config, error) {
	cfg := Config{}

	if err := envstruct.Load(&cfg); err != nil {
		return Config{}, err
	}

	return cfg, nil
}
