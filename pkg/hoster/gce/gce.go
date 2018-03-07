// This will auth with the current instance's service account (if any).
// Use GOOGLE_APPLICATION_CREDENTIALS environment variable to specify
// a service account key file to authenticate to the API.
// See https://cloud.google.com/docs/authentication/.
package gce

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/bpineau/cloud-floating-ip/config"
	"golang.org/x/oauth2/google"
	compute "google.golang.org/api/compute/v1"

	"cloud.google.com/go/compute/metadata"
	//"golang.org/x/oauth2/google"
	//"google.golang.org/api/compute/v0.beta"
)

const (
	instanceSelfLink = `https://www.googleapis.com/compute/v1/projects/%s/zones/%s/instances/%s`
)

// Hoster represents an hosting provider (here, gce)
type Hoster struct {
	conf     *config.CfiConfig
	network  string // XXX this should also be a cli option
	rname    string // this too
	ctx      context.Context
	selflink string
	client   *http.Client
	svc      *compute.Service
}

// Init prepare a gce hoster for usage
func (h *Hoster) Init(conf *config.CfiConfig) {
	var err error
	h.conf = conf

	// XXX tester si pas ongce et qu'il manque une var: abort direct

	if h.conf.Project == "" {
		h.conf.Project, err = metadata.ProjectID()
		if err != nil {
			log.Fatalf("Failed to guess project id: %v", err)
		}
	}

	if h.conf.Instance == "" {
		h.conf.Instance, err = metadata.InstanceName()
		if err != nil {
			log.Fatalf("Failed to guess instance id: %v", err)
		}
	}

	if h.conf.Zone == "" {
		h.conf.Zone, err = metadata.Zone()
		if err != nil {
			log.Fatalf("Failed to guess instance zone: %v", err)
		}
	}

	h.network, err = metadata.Get("instance/network-interfaces/0/network")
	if err != nil {
		log.Fatalf("Failed to guess network link: %v", err)
	}

	h.rname = strings.Replace("rule-for-"+h.conf.IP, ".", "-", -1)

	h.selflink = fmt.Sprintf(instanceSelfLink, h.conf.Project, h.conf.Zone, h.conf.Instance)

	h.ctx = context.Background()

	h.client, err = google.DefaultClient(h.ctx, compute.CloudPlatformScope)
	if err != nil {
		log.Fatal(err)
	}

	h.svc, err = compute.New(h.client)
	if err != nil {
		log.Fatal(err)
	}
}

// OnThisHoster returns true when we run on an gce instance
func (h *Hoster) OnThisHoster() bool {
	return metadata.OnGCE()
}

// Preempt takes over the floating IP address
func (h *Hoster) Preempt() error {
	if h.Status() { // we're already primary/owner
		return nil
	}

	rb := &compute.Route{
		Name:            h.rname,
		NextHopInstance: h.selflink,
		Network:         h.network,
		DestRange:       h.conf.IP,
	}

	// try to delete any possible pre-existing route, but don't obsess on it
	_, _ = h.svc.Routes.Delete(h.conf.Project, h.rname).Context(h.ctx).Do()

	_, err := h.svc.Routes.Insert(h.conf.Project, rb).Context(h.ctx).Do()
	if err != nil {
		log.Fatal("Failed to create the route: %v", err)
	}

	return nil
}

// Status returns true if the floating IP address route to the instance
func (h *Hoster) Status() bool {
	resp, err := h.svc.Routes.Get(h.conf.Project, h.rname).Context(h.ctx).Do()
	if err != nil {
		log.Fatal(err)
	}

	return resp.NextHopInstance == h.selflink
}
