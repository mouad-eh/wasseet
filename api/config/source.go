package config

type Source interface {
	Load() (Config, error)
}
