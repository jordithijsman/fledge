package main

import (
	"gitlab.ilabt.imec.be/fledge/service/cmd/fledge/internal/provider"
	"gitlab.ilabt.imec.be/fledge/service/cmd/fledge/internal/provider/mock"
	"gitlab.ilabt.imec.be/fledge/service/pkg/fledge"
	backend "gitlab.ilabt.imec.be/fledge/service/pkg/provider"
)

func registerMock(s *provider.Store) {
	/* #nosec */
	s.Register("mock", func(cfg provider.InitConfig) (provider.Provider, error) { //nolint:errcheck
		return mock.NewMockProvider(
			cfg.ConfigPath,
			cfg.NodeName,
			cfg.OperatingSystem,
			cfg.InternalIP,
			cfg.DaemonPort,
		)
	})
}

func registerBackend(s *provider.Store) {
	s.Register("backend", func(cfg provider.InitConfig) (provider.Provider, error) {
		return backend.NewProvider(
			cfg.ConfigPath,
			cfg.NodeName,
			cfg.OperatingSystem,
			cfg.InternalIP,
			cfg.DaemonPort,
		)
	})
}

func registerBroker(s *provider.Store) {
	s.Register("broker", func(cfg provider.InitConfig) (provider.Provider, error) {
		return fledge.NewBrokerProvider(
			cfg.ConfigPath,
			cfg.NodeName,
			cfg.OperatingSystem,
			cfg.InternalIP,
			cfg.DaemonPort,
		)
	})
}
