package aws

import (
	"fmt"
	"log"

	"github.com/bpineau/cloud-floating-ip/config"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
)

// Hoster represents an hosting provider (here, AWS)
type Hoster struct {
	conf   *config.CfiConfig
	sess   *session.Session
	ec2s   *ec2.EC2
	routes []*ec2.RouteTable
	enid   *string
	cidr   *string
	vpc    string
	myip   string
}

type routeStatus int

const (
	rsAbsent routeStatus = iota
	rsWrongTarget
	rsCorrectTarget
)

// Init prepare an aws hoster for usage
func (h *Hoster) Init(conf *config.CfiConfig) {
	h.conf = conf
	err := h.checkMissingParam()
	if err != nil {
		log.Fatalf("Missing param: %v", err)
	}

	h.sess, err = session.NewSession(aws.NewConfig().WithMaxRetries(3))
	if err != nil {
		log.Fatalf("Failed to initialize an AWS session: %v", err)
	}

	metadata := ec2metadata.New(h.sess)

	if h.conf.Region == "" {
		h.conf.Region, err = metadata.Region()
		if err != nil {
			log.Fatalf("Failed to collect region from instance metadata: %v", err)
		}
	}

	h.sess.Config.Region = aws.String(h.conf.Region)

	if h.conf.Instance == "" {
		h.conf.Instance, err = metadata.GetMetadata("instance-id")
		if err != nil {
			log.Fatalf("Failed to collect instanceid from instance metadata: %v", err)
		}
	}

	h.ec2s = ec2.New(h.sess)

	err = h.getNetworkInfo()
	if err != nil {
		log.Fatalf("Failed to collect network infos: %v", err)
	}
}

func (h *Hoster) getNetworkInfo() error {
	instance, err := h.ec2s.DescribeInstances(&ec2.DescribeInstancesInput{
		InstanceIds: []*string{aws.String(h.conf.Instance)}},
	)
	if err != nil {
		return fmt.Errorf("Failed to DescribeInstances: %v", err)
	}

	// TODO: support instances with several interfaces
	if len(instance.Reservations[0].Instances[0].NetworkInterfaces) != 1 {
		return fmt.Errorf("For now, we don't support more than one interface")
	}

	eni := instance.Reservations[0].Instances[0].NetworkInterfaces[0]
	h.enid = eni.NetworkInterfaceId
	h.vpc = *eni.VpcId
	h.myip = *eni.PrivateIpAddress

	cidr := h.conf.IP + "/32"
	h.cidr = &cidr

	input := &ec2.DescribeRouteTablesInput{
		Filters: []*ec2.Filter{
			&ec2.Filter{
				Name:   aws.String("vpc-id"),
				Values: []*string{aws.String(h.vpc)},
			},
		},
	}

	routes, err := h.ec2s.DescribeRouteTables(input)
	if err != nil {
		return fmt.Errorf("Failed to DescribeRouteTables: %v", err)
	}

	h.routes = routes.RouteTables

	return nil
}

// OnThisHoster returns true when we run on an gce instance
func (h *Hoster) OnThisHoster() bool {
	sess, err := session.NewSession(aws.NewConfig().WithMaxRetries(3))
	if err != nil {
		return false
	}

	h.sess = sess
	metadata := ec2metadata.New(sess)

	return metadata.Available()
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

	// contrary to GCE, an EC2 VPC can have several routes tables
	for _, table := range h.routes {
		var err error

		if h.conf.NoMain && isMainTableAssociated(table) {
			continue
		}

		status := isRouteInTable(table, h.cidr, h.enid, h.conf.Instance)

		switch status {
		case rsCorrectTarget:
			continue
		case rsAbsent:
			err = h.addRouteInTable(table, h.cidr, h.enid)
		case rsWrongTarget:
			err = h.replaceRouteInTable(table, h.cidr, h.enid)
		}

		if err != nil {
			log.Fatalf("Failed to create a route: %v", err)
		}
	}

	return nil
}

// Status returns true if the floating IP address route to the instance
func (h *Hoster) Status() bool {
	for _, table := range h.routes {
		if h.conf.NoMain && isMainTableAssociated(table) {
			continue
		}

		if isRouteInTable(table, h.cidr, h.enid, h.conf.Instance) != rsCorrectTarget {
			return false
		}
	}

	return true
}

// Destroy remove route(s) to the IP from our VPC
func (h *Hoster) Destroy() error {
	for _, table := range h.routes {
		if h.conf.NoMain && isMainTableAssociated(table) {
			continue
		}

		status := isRouteInTable(table, h.cidr, h.enid, h.conf.Instance)
		if status == rsAbsent {
			continue
		}

		route := &ec2.DeleteRouteInput{
			RouteTableId:         table.RouteTableId,
			DestinationCidrBlock: h.cidr,
		}

		if !h.conf.Quiet {
			fmt.Printf("Deleting route to %s from %s table\n",
				*h.cidr, *table.RouteTableId)
		}

		if h.conf.DryRun {
			continue
		}

		_, err := h.ec2s.DeleteRoute(route)
		if err != nil {
			return fmt.Errorf("Failed to delete route: %v", err)
		}
	}

	return nil
}

func (h *Hoster) checkMissingParam() error {
	if h.OnThisHoster() {
		return nil
	}

	if h.conf.Region == "" || h.conf.Instance == "" {
		return fmt.Errorf("%s %s", "when not running on a instance, ",
			"you must provide region, and instanceid")
	}

	return nil
}

func isMainTableAssociated(table *ec2.RouteTable) bool {
	for _, assoc := range table.Associations {
		if *assoc.Main {
			return true
		}
	}
	return false
}

func isRouteInTable(table *ec2.RouteTable, cidr *string, eni *string, instance string) routeStatus {
	for _, route := range table.Routes {
		if route.DestinationCidrBlock == nil {
			continue
		}

		if *route.DestinationCidrBlock != *cidr {
			continue
		}

		if route.InstanceId != nil && instance != "" && *route.InstanceId == instance {
			return rsCorrectTarget
		}

		if route.NetworkInterfaceId != nil && eni != nil && *route.NetworkInterfaceId == *eni {
			return rsCorrectTarget
		}

		return rsWrongTarget
	}

	return rsAbsent
}

func (h *Hoster) addRouteInTable(table *ec2.RouteTable, cidr *string, eni *string) error {
	route := &ec2.CreateRouteInput{
		RouteTableId:         table.RouteTableId,
		DestinationCidrBlock: cidr,
		NetworkInterfaceId:   eni,
	}

	if !h.conf.Quiet {
		fmt.Printf("Creating route to %s via ENI %s in table %s\n",
			*cidr, *eni, *table.RouteTableId)
	}

	if h.conf.DryRun {
		return nil
	}

	_, err := h.ec2s.CreateRoute(route)
	return err
}

func (h *Hoster) replaceRouteInTable(table *ec2.RouteTable, cidr *string, eni *string) error {
	route := &ec2.ReplaceRouteInput{
		RouteTableId:         table.RouteTableId,
		DestinationCidrBlock: cidr,
		NetworkInterfaceId:   eni,
	}

	if !h.conf.Quiet {
		fmt.Printf("Replacing route to %s via ENI %s in table %s\n",
			*cidr, *eni, *table.RouteTableId)
	}

	if h.conf.DryRun {
		return nil
	}

	_, err := h.ec2s.ReplaceRoute(route)
	return err
}
