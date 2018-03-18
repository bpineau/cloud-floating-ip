// This will auth using the current instance's service account (if any).
// Or you can use GOOGLE_APPLICATION_CREDENTIALS environment variable
// to specify a json service account key file to authenticate to the API.
// See https://cloud.google.com/docs/authentication/.

// Package gce implement floating IP for GCE/GCP instances.
package gce

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/bpineau/cloud-floating-ip/config"
	"github.com/bpineau/cloud-floating-ip/pkg/log"

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
	log      log.Logger
	network  string
	rname    string
	selflink string
}

// Init prepare a gce hoster for usage
func (h *Hoster) Init(conf *config.CfiConfig, logger log.Logger) {
	var err error
	ctx := context.Background()
	h.conf = conf
	h.log = logger

	err = h.checkMissingParam()
	if err != nil {
		h.log.Fatalf("Missing parameter: %v\n", err)
	}

	h.conf.Project, err = h.getProject()
	if err != nil {
		h.log.Fatalf("Failed to guess project id: %v\n", err)
	}

	h.conf.Instance, err = h.getInstance()
	if err != nil {
		h.log.Fatalf("Failed to guess instance id: %v\n", err)
	}

	h.conf.Zone, err = h.getZone()
	if err != nil {
		h.log.Fatalf("Failed to guess instance zone: %v\n", err)
	}

	h.rname = strings.Replace(routePrefix+h.conf.IP, ".", "-", -1)
	h.selflink = fmt.Sprintf(instanceSelfLink, h.conf.Project, h.conf.Zone, h.conf.Instance)
	h.ctx = &ctx

	h.client, err = google.DefaultClient(*h.ctx, compute.CloudPlatformScope)
	if err != nil {
		h.log.Fatalf("Failed to get default client %s\n", err)
	}

	h.svc, err = compute.New(h.client)
	if err != nil {
		h.log.Fatalf("Failed to instantiate a compute client: %s\n", err)
	}

	h.network, err = h.getNetwork()
	if err != nil {
		h.log.Fatalf("Failed to collect network infos: %v\n", err)
	}
}

func (h *Hoster) getProject() (string, error) {
	if h.conf.Project != "" {
		return h.conf.Project, nil
	}

	return metadata.ProjectID()
}

func (h *Hoster) getInstance() (string, error) {
	if h.conf.Instance != "" {
		return h.conf.Instance, nil
	}

	return metadata.InstanceName()
}

func (h *Hoster) getZone() (string, error) {
	if h.conf.Zone != "" {
		return h.conf.Zone, nil
	}

	return metadata.Zone()
}

func (h *Hoster) getNetwork() (string, error) {
	inst, err := h.svc.Instances.Get(h.conf.Project, h.conf.Zone, h.conf.Instance).Context(*h.ctx).Do()
	if err != nil {
		return "", fmt.Errorf("Failed to read instance attributes: %v", err)
	}

	if len(inst.NetworkInterfaces) < 1 {
		return "", fmt.Errorf("can't find any interface on instance %s", h.conf.Instance)
	}

	if len(inst.NetworkInterfaces) != 1 && h.conf.Iface == "" && h.conf.Subnet == "" && h.conf.TargetIP == "" {
		return "", fmt.Errorf("the instance %s has more than one interface, %s",
			h.conf.Instance, "please specify an interface, target IP, or subnet ID.")
	}

	if h.conf.Iface != "" {
		return h.getNetworkByInterface(h.conf.Iface, inst.NetworkInterfaces)
	}

	if h.conf.Subnet != "" {
		return h.getNetworkBySubnet(h.conf.Subnet, inst.NetworkInterfaces)
	}

	if h.conf.TargetIP != "" {
		return h.getNetworkByTargetIP(h.conf.TargetIP, inst.NetworkInterfaces)
	}

	return inst.NetworkInterfaces[0].Network, nil
}

func (h *Hoster) getNetworkByInterface(name string, ifaces []*compute.NetworkInterface) (string, error) {
	for _, iface := range ifaces {
		if iface.Name == name {
			return iface.Network, nil
		}
	}

	return "", fmt.Errorf("can't find an interface named %s", name)
}

func (h *Hoster) getNetworkBySubnet(name string, ifaces []*compute.NetworkInterface) (string, error) {
	for _, iface := range ifaces {
		subnet := strings.Split(iface.Subnetwork, "/")
		if subnet[len(subnet)-1] == name {
			return iface.Network, nil
		}
	}

	return "", fmt.Errorf("can't find an interface on subnet %s", name)
}

func (h *Hoster) getNetworkByTargetIP(name string, ifaces []*compute.NetworkInterface) (string, error) {
	for _, iface := range ifaces {
		if iface.NetworkIP == name {
			return iface.Network, nil
		}
	}

	return "", fmt.Errorf("can't find an interface with IP %s", name)
}

// OnThisHoster returns true when we run on an gce instance
func (h *Hoster) OnThisHoster() bool {
	return metadata.OnGCE()
}

// Preempt takes over the floating IP address
func (h *Hoster) Preempt() error {
	if h.Status() {
		h.log.Infof("Already primary, nothing to do\n")
		return nil
	}

	h.log.Infof("Preempting %s route(s)\n", h.conf.IP)

	rb := &compute.Route{
		Name:            h.rname,
		NextHopInstance: h.selflink,
		Network:         h.network,
		DestRange:       h.conf.IP,
	}

	// There's no "update" or "replace" in GCP routes API.
	err := h.Destroy()
	if err != nil {
		h.log.Fatalf("Failed to delete the route: %v\n", err)
	}

	h.log.Infof("Creating a route %s to %s via %s on %s network\n",
		h.rname, h.conf.IP, h.selflink, h.network)

	if h.conf.DryRun {
		return nil
	}

	err = h.blockingWait(h.svc.Routes.Insert(h.conf.Project, rb).Do())
	if err != nil {
		h.log.Fatalf("Failed to create the route: %v\n", err)
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

	h.log.Fatalf("Failed to get route status: %v\n", err)

	return false
}

// Destroy remove route to the IP from our VPC
func (h *Hoster) Destroy() error {
	h.log.Infof("Deleting route to %s from %s network\n", h.conf.IP, h.network)

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
