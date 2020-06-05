package docker

import (
	"context"

	"github.com/docker/buildx/driver"
	"github.com/docker/buildx/util/progress"
	"github.com/moby/buildkit/client"
)

type Driver struct {
	driver.InitConfig
	factory driver.Factory
	address string
}

func (d *Driver) Bootstrap(ctx context.Context, l progress.Logger) error {
	return nil
}

func (d *Driver) Info(ctx context.Context) (*driver.Info, error) {
	return &driver.Info{
		Status: driver.Running,
	}, nil
}

func (d *Driver) Stop(ctx context.Context, force bool) error {
	return nil
}

func (d *Driver) Rm(ctx context.Context, force bool) error {
	return nil
}

func (d *Driver) Client(ctx context.Context) (*client.Client, error) {
	return client.New(ctx, d.address, nil)
}

func (d *Driver) Factory() driver.Factory {
	return d.factory
}

func (d *Driver) Features() map[driver.Feature]bool {
	return map[driver.Feature]bool{
		driver.OCIExporter:    true,
		driver.DockerExporter: true,

		driver.CacheExport:   true,
		driver.MultiPlatform: true,
	}
}

func (d *Driver) Config() driver.InitConfig {
	return d.InitConfig
}

func (d *Driver) IsMobyDriver() bool {
	return false
}
