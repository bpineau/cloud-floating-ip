package gce

import (
	//"fmt"
	"github.com/bpineau/cloud-floating-ip/config"
	//"github.com/bpineau/cloud-floating-ip/pkg/hoster"
)

// Hoster represents an hosting provider (here, gce)
type Hoster struct{}

// OnThisHoster returns true when we run on an gce instance
func (h *Hoster) OnThisHoster(conf *config.CfiConfig) bool {
	return true
}

// Preempt takes over the floating IP address
func (h *Hoster) Preempt(conf *config.CfiConfig) error {
	return nil
}

// Status returns true if the floating IP address route to the instance
func (h *Hoster) Status(conf *config.CfiConfig) (bool, error) {
	return true, nil
}
