// Package gce implement floating IP for GCE/GCP instances.
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
	"time"

	"github.com/bpineau/cloud-floating-ip/config"

	"cloud.google.com/go/compute/metadata"
	"golang.org/x/oauth2/google"
	compute "google.golang.org/api/compute/v1"
	"google.golang.org/api/googleapi"
)

const (
	instanceSelfLink = `https://www.googleapis.com/compute/v1/projects/%s/zones/%s/instances/%s`
	routePrefix      = `cloud-floating-ip-rule-for-`
)

// Hoster represents an hosting provider (here, gce)
type Hoster struct {
	conf     *config.CfiConfig
	network  string
	rname    string
	ctx      context.Context
	selflink string
	client   *http.Client
	svc      *compute.Service
}

// Init prepare a gce hoster for usage
func (h *Hoster) Init(conf *config.CfiConfig) {
	var err error
	h.conf = conf

	err = h.checkMissingParam()
	if err != nil {
		log.Fatalf("Missing param: %v", err)
	}

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

	h.rname = strings.Replace(routePrefix+h.conf.IP, ".", "-", -1)
	h.selflink = fmt.Sprintf(instanceSelfLink, h.conf.Project, h.conf.Zone, h.conf.Instance)
	h.ctx = context.Background() // XXX set a timeout

	h.client, err = google.DefaultClient(h.ctx, compute.CloudPlatformScope)
	if err != nil {
		log.Fatal(err)
	}

	h.svc, err = compute.New(h.client)
	if err != nil {
		log.Fatal(err)
	}

	h.network = h.getNetworkInterface()
}

func (h *Hoster) getNetworkInterface() string {
	inst, err := h.svc.Instances.Get(h.conf.Project, h.conf.Zone, h.conf.Instance).Context(h.ctx).Do()
	if err != nil {
		log.Fatalf("Failed to guess network link: %v", err)
	}

	// XXX deal with instances with more than one interface
	if len(inst.NetworkInterfaces) != 1 {
		log.Fatal("For now, we don't support more than one interface")
	}

	return inst.NetworkInterfaces[0].Network
}

// OnThisHoster returns true when we run on an gce instance
func (h *Hoster) OnThisHoster() bool {
	return metadata.OnGCE()
}

// Preempt takes over the floating IP address
func (h *Hoster) Preempt() error {
	// if we're already primary/owner, do nothing: we're idempotent.
	if h.Status() {
		return nil
	}

	rb := &compute.Route{
		Name:            h.rname,
		NextHopInstance: h.selflink,
		Network:         h.network,
		DestRange:       h.conf.IP,
	}

	// the api don't offer updates: if something already exists, delete it
	err := h.blockingWait(h.svc.Routes.Delete(h.conf.Project, h.rname).Do())
	if err != nil {
		apierr, ok := err.(*googleapi.Error)

		if !ok {
			log.Fatalf("Failed to delete existing route and read error: %v", err)
		}

		if apierr.Code != 404 {
			log.Fatalf("Failed to delete an existing rule: %v\n", err)
		}
	}

	err = h.blockingWait(h.svc.Routes.Insert(h.conf.Project, rb).Do())
	if err != nil {
		log.Fatalf("Failed to create the route: %v", err)
	}

	return nil
}

// Status returns true if the floating IP address route to the instance
func (h *Hoster) Status() bool {
	resp, err := h.svc.Routes.Get(h.conf.Project, h.rname).Context(h.ctx).Do()

	if err == nil {
		return resp.NextHopInstance == h.selflink
	}

	// route not found is ok, means we don't "own" the IP
	if apierr, ok := err.(*googleapi.Error); ok {
		if apierr.Code == 404 {
			return false
		}
	}

	log.Fatalf("Failed to get route status: %v", err)

	return false
}

func (h *Hoster) checkMissingParam() error {
	if h.OnThisHoster() {
		return nil
	}

	if h.conf.Zone == "" || h.conf.Instance == "" || h.conf.Project == "" {
		return fmt.Errorf("%s %s", "when not running this on a instance, ",
			"you must provide project, zone and instance names")
	}

	return nil
}

func (h *Hoster) blockingWait(op *compute.Operation, err error) error {
	if err != nil {
		return err
	}

	for i := 0; i < 120; i++ {
		operation, err := h.svc.GlobalOperations.Get(h.conf.Project, op.Name).Do()
		if err != nil {
			return err
		}

		if operation.Status == "DONE" {
			return nil
		}

		if operation.Error != nil {
			return fmt.Errorf("Operation failed: %v", operation.Error)
		}

		time.Sleep(time.Second)
	}

	return fmt.Errorf("timeout waiting for %s to finish", op.Name)
}
