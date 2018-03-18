package hoster

import (
	"errors"

	"github.com/bpineau/cloud-floating-ip/config"
	"github.com/bpineau/cloud-floating-ip/pkg/hoster/aws"
	"github.com/bpineau/cloud-floating-ip/pkg/hoster/gce"
	"github.com/bpineau/cloud-floating-ip/pkg/log"
)

// Hoster represents an hosting provider (aws or gce)
type Hoster interface {
	Init(conf *config.CfiConfig, logger log.Logger)
	OnThisHoster() bool
	Preempt() error
	Status() bool
	Destroy() error
}

var allHosters = map[string]Hoster{
	"aws": &aws.Hoster{},
	"gce": &gce.Hoster{},
}

// GuessHoster returns the hoster described by name or found in instance's metadata
func GuessHoster(name string) (Hoster, error) {
	var h Hoster

	if name != "" {
		if host, ok := allHosters[name]; ok {
			return host, nil
		}

		return nil, errors.New("hoster not supported: " + name)
	}

	for _, h = range allHosters {
		if h.OnThisHoster() {
			return h, nil
		}
	}

	return nil, errors.New("failed to guess the current host's hoster (neither aws or gce?)")
}
