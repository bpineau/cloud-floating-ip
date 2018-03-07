package hoster

import (
	"github.com/bpineau/cloud-floating-ip/config"
	"github.com/bpineau/cloud-floating-ip/pkg/hoster/gce"
)

// Hoster represents an hosting provider (aws or gce)
type Hoster interface {
	OnThisHoster(conf *config.CfiConfig) bool
	Preempt(conf *config.CfiConfig) error
	Status(conf *config.CfiConfig) (bool, error)
}

// Hosters holds all known Hoster
var Hosters = []Hoster{
	&gce.Hoster{},
}
