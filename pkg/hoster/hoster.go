package hoster

import (
	"github.com/bpineau/cloud-floating-ip/config"
	"github.com/bpineau/cloud-floating-ip/pkg/hoster/gce"
)

// Hoster represents an hosting provider (aws or gce)
type Hoster interface {
	Init(conf *config.CfiConfig)
	OnThisHoster() bool
	Preempt() error
	Status() bool
}

// Hosters holds all known Hoster
var Hosters = []Hoster{
	&gce.Hoster{},
}
