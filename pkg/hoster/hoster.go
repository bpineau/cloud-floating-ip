package hoster

import (
	"errors"

	"github.com/bpineau/cloud-floating-ip/config"
	"github.com/bpineau/cloud-floating-ip/pkg/hoster/aws"
	"github.com/bpineau/cloud-floating-ip/pkg/hoster/gce"
)

// Hoster represents an hosting provider (aws or gce)
type Hoster interface {
	Init(conf *config.CfiConfig)
	OnThisHoster() bool
	Preempt() error
	Status() bool
	Destroy() error
}

// Hosters holds all known Hoster
var Hosters = map[string]Hoster{
	"aws": &aws.Hoster{},
	"gce": &gce.Hoster{},
}

// GuessHoster returns the hoster described by name or found in instance's metadata
func GuessHoster(name string) (Hoster, error) {
	var h Hoster

	if name != "" {
		if host, ok := Hosters[name]; ok {
			return host, nil
		}

		return nil, errors.New("hoster not supported: " + name)
	}

	for _, h = range Hosters {
		if h.OnThisHoster() {
			return h, nil
		}
	}

	return nil, errors.New("failed to guess the current host's hoster (neither aws or gce?)")
}
