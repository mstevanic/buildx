package docker

import (
	"context"

	"github.com/docker/buildx/driver"
	dockerclient "github.com/docker/docker/client"
	"github.com/pkg/errors"
)

const prioritySupported = 30
const priorityUnsupported = 70

func init() {
	driver.Register(&factory{})
}

type factory struct {
}

func (*factory) Name() string {
	return "buildkit-external"
}

func (*factory) Usage() string {
	return "buildkit-external"
}

func (*factory) Priority(ctx context.Context, api dockerclient.APIClient) int {
	if api == nil {
		return priorityUnsupported
	}
	return prioritySupported
}

func (f *factory) New(ctx context.Context, cfg driver.InitConfig) (driver.Driver, error) {
	d := &Driver{factory: f, InitConfig: cfg}
	for k, v := range cfg.DriverOpts {
		switch {
		case k == "address":
			d.address = v
		default:
			return nil, errors.Errorf("invalid driver option %s for buildkit-external driver", k)
		}
	}

	return d, nil
}

func (f *factory) AllowsInstances() bool {
	return true
}
