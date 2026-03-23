package config

import (
	"time"

	"github.com/hashicorp/hcl/v2/hclsimple"
)

type Config struct {
	Location     Location     `hcl:"location,block"`
	PollInterval string       `hcl:"poll_interval"`
	Redis        RedisConfig  `hcl:"redis,block"`
	Postgres     PostgresConfig `hcl:"postgres,block"`
}

type Location struct {
	Lat      float64 `hcl:"lat"`
	Lon      float64 `hcl:"lon"`
	RadiusKm float64 `hcl:"radius_km"`
}

type RedisConfig struct {
	Addr     string `hcl:"addr"`
	Password string `hcl:"password,optional"`
	DB       int    `hcl:"db,optional"`
}

type PostgresConfig struct {
	DSN string `hcl:"dsn"`
}

func (c *Config) PollDuration() (time.Duration, error) {
	return time.ParseDuration(c.PollInterval)
}

func Load(path string) (*Config, error) {
	var cfg Config
	if err := hclsimple.DecodeFile(path, nil, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}
