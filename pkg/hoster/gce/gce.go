// This will auth using the current instance's service account (if any).
// Or you can use GOOGLE_APPLICATION_CREDENTIALS environment variable
// to specify a json service account key file to authenticate to the API.
// See https://cloud.google.com/docs/authentication/.

// Package gce implement floating IP for GCE/GCP instances.
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
	client   *http.Client
	svc      *compute.Service
	ctx      *context.Context
	network  string
	rname    string
	selflink string
}

// Init prepare a gce hoster for usage
func (h *Hoster) Init(conf *config.CfiConfig) {
	var err error
	ctx := context.Background()
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
	h.ctx = &ctx

	h.client, err = google.DefaultClient(*h.ctx, compute.CloudPlatformScope)
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
	inst, err := h.svc.Instances.Get(h.conf.Project, h.conf.Zone, h.conf.Instance).Context(*h.ctx).Do()
	if err != nil {
		log.Fatalf("Failed to guess network link: %v", err)
	}

	// TODO: support instances with several interfaces
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
	if h.Status() {
		if !h.conf.Quiet {
			fmt.Printf("Already primary, nothing to do\n")
		}
		return nil
	}

	if !h.conf.Quiet {
		fmt.Printf("Preempting %s route(s)\n", h.conf.IP)
	}

	rb := &compute.Route{
		Name:            h.rname,
		NextHopInstance: h.selflink,
		Network:         h.network,
		DestRange:       h.conf.IP,
	}

	// There's no "update" or "replace" in GCP routes API.
	err := h.Destroy()
	if err != nil {
		log.Fatalf("Failed to delete the route: %v", err)
	}

	if !h.conf.Quiet {
		fmt.Printf("Creating a route %s to %s via %s on %s network\n",
			h.rname, h.conf.IP, h.selflink, h.network)
	}

	if h.conf.DryRun {
		return nil
	}

	err = h.blockingWait(h.svc.Routes.Insert(h.conf.Project, rb).Do())
	if err != nil {
		log.Fatalf("Failed to create the route: %v", err)
	}

	return nil
}

// Status returns true if the floating IP address route to the instance
func (h *Hoster) Status() bool {
	resp, err := h.svc.Routes.Get(h.conf.Project, h.rname).Context(*h.ctx).Do()

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

// Destroy remove route to the IP from our VPC
func (h *Hoster) Destroy() error {
	if !h.conf.Quiet {
		fmt.Printf("Deleting route to %s from %s network\n",
			h.conf.IP, h.network)
	}

	if h.conf.DryRun {
		return nil
	}

	err := h.blockingWait(h.svc.Routes.Delete(h.conf.Project, h.rname).Do())
	if err == nil {
		return nil
	}

	apierr, ok := err.(*googleapi.Error)

	if !ok {
		return fmt.Errorf("failed to delete a route and read error: %v", err)
	}

	if apierr.Code != 404 {
		return fmt.Errorf("failed to delete an existing route: %v", err)
	}

	return nil
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
