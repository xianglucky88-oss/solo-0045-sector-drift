package service

import (
	"example.local/sector-drift/internal/domain"
	"example.local/sector-drift/internal/vault"
)

type Service struct {
	vault    vault.Vault
	slots    chan struct{}
	sessions *sessionHub
}

func New(path string) *Service {
	return &Service{
		vault:    vault.Vault{Dir: path},
		slots:    make(chan struct{}, 2),
		sessions: newSessionHub(),
	}
}
func compile(body domain.Request) ([]byte, error) { return domain.Build(body) }
