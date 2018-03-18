package aws

import (
	"fmt"

	"github.com/bpineau/cloud-floating-ip/config"
	"github.com/bpineau/cloud-floating-ip/pkg/log"

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
	log    log.Logger
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

	inuse = "in-use"
)

// Init prepare an aws hoster for usage
func (h *Hoster) Init(conf *config.CfiConfig, logger log.Logger) {
	h.conf = conf
	h.log = logger
	err := h.checkMissingParam()
	if err != nil {
		h.log.Fatalf("Missing param: %v\n", err)
	}

	h.sess, err = session.NewSession(aws.NewConfig().WithMaxRetries(3))
	if err != nil {
		h.log.Fatalf("Failed to initialize an AWS session: %v\n", err)
	}

	metadata := ec2metadata.New(h.sess)

	if h.conf.Region == "" {
		h.conf.Region, err = metadata.Region()
		if err != nil {
			h.log.Fatalf("Failed to collect region from instance metadata: %v\n", err)
		}
	}

	h.sess.Config.Region = aws.String(h.conf.Region)

	if h.conf.Instance == "" {
		h.conf.Instance, err = metadata.GetMetadata("instance-id")
		if err != nil {
			h.log.Fatalf("Failed to collect instanceid from instance metadata: %v\n", err)
		}
	}

	h.ec2s = ec2.New(h.sess)

	err = h.getNetworkInfo()
	if err != nil {
		h.log.Fatalf("Failed to collect network infos: %v\n", err)
	}
}

func (h *Hoster) getNetworkInfo() error {

	eni, err := h.getNetworkInterface()
	if err != nil {
		return fmt.Errorf("failed to find the target interface: %v", err)
	}

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
		return fmt.Errorf("failed to DescribeRouteTables: %v", err)
	}

	h.routes = h.filterRouteTables(routes.RouteTables)
	if len(h.routes) == 0 {
		return fmt.Errorf("no route table left after filtering")
	}

	return nil
}

// find the target ENI/interface ; if we're multihomed (have several external
// interfaces), we'll filter using the user-provided interface or subnet name.
func (h *Hoster) getNetworkInterface() (*ec2.InstanceNetworkInterface, error) {
	instance, err := h.ec2s.DescribeInstances(&ec2.DescribeInstancesInput{
		InstanceIds: []*string{aws.String(h.conf.Instance)}},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to DescribeInstances: %v", err)
	}

	ifaces := instance.Reservations[0].Instances[0].NetworkInterfaces
	if len(ifaces) < 1 {
		return nil, fmt.Errorf("instance %s doesn't have a network interface",
			h.conf.Instance)
	}

	if len(ifaces) != 1 && h.conf.Iface == "" && h.conf.Subnet == "" && h.conf.TargetIP == "" {
		return nil, fmt.Errorf("the instance %s has more than one interface, %s",
			h.conf.Instance, "please specify an interface, target IP, or subnet ID.")
	}

	if h.conf.Iface != "" {
		return h.getNetworkInterfaceByName(h.conf.Iface, ifaces)
	}

	if h.conf.Subnet != "" {
		return h.getNetworkInterfaceBySubnet(h.conf.Subnet, ifaces)
	}

	if h.conf.TargetIP != "" {
		return h.getNetworkInterfaceByTargetIP(h.conf.TargetIP, ifaces)
	}

	return ifaces[0], nil
}

func (h *Hoster) getNetworkInterfaceByName(name string, ifaces []*ec2.InstanceNetworkInterface) (*ec2.InstanceNetworkInterface, error) {
	for _, iface := range ifaces {
		if iface.NetworkInterfaceId == nil || iface.SubnetId == nil || iface.PrivateIpAddress == nil {
			continue
		}

		if iface.Status == nil || *iface.Status != inuse {
			continue
		}

		if *iface.NetworkInterfaceId == name {
			return iface, nil
		}
	}

	return nil, fmt.Errorf("can't find the interface %s on instance %s", name, h.conf.Instance)
}

func (h *Hoster) getNetworkInterfaceBySubnet(name string, ifaces []*ec2.InstanceNetworkInterface) (*ec2.InstanceNetworkInterface, error) {
	for _, iface := range ifaces {
		if iface.NetworkInterfaceId == nil || iface.SubnetId == nil || iface.PrivateIpAddress == nil {
			continue
		}

		if iface.Status == nil || *iface.Status != inuse {
			continue
		}

		if *iface.SubnetId == name {
			return iface, nil
		}
	}

	return nil, fmt.Errorf("can't find an interface on subnet %s for instance %s", name, h.conf.Instance)
}

func (h *Hoster) getNetworkInterfaceByTargetIP(name string, ifaces []*ec2.InstanceNetworkInterface) (*ec2.InstanceNetworkInterface, error) {
	for _, iface := range ifaces {
		if iface.NetworkInterfaceId == nil || iface.SubnetId == nil || iface.PrivateIpAddress == nil {
			continue
		}

		if iface.Status == nil || *iface.Status != inuse {
			continue
		}

		if *iface.PrivateIpAddress == name {
			return iface, nil
		}
	}

	return nil, fmt.Errorf("can't find an interface with IP %s for instance %s", name, h.conf.Instance)
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
		h.log.Infof("Already primary, nothing to do\n")
		return nil
	}

	h.log.Infof("Preempting %s route(s)\n", h.conf.IP)

	for _, table := range h.routes {
		var err error

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
			h.log.Fatalf("Failed to create a route: %v\n", err)
		}
	}

	return nil
}

// Status returns true if the floating IP address route to the instance
func (h *Hoster) Status() bool {
	for _, table := range h.routes {
		if isRouteInTable(table, h.cidr, h.enid, h.conf.Instance) != rsCorrectTarget {
			return false
		}
	}

	return true
}

// Destroy remove route(s) to the IP from our VPC
func (h *Hoster) Destroy() error {
	for _, table := range h.routes {
		status := isRouteInTable(table, h.cidr, h.enid, h.conf.Instance)
		if status == rsAbsent {
			continue
		}

		route := &ec2.DeleteRouteInput{
			RouteTableId:         table.RouteTableId,
			DestinationCidrBlock: h.cidr,
		}

		h.log.Infof("Deleting route to %s from %s table\n",
			*h.cidr, *table.RouteTableId)

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

	h.log.Infof("Creating route to %s via ENI %s in table %s\n",
		*cidr, *eni, *table.RouteTableId)

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

	h.log.Infof("Replacing route to %s via ENI %s in table %s\n",
		*cidr, *eni, *table.RouteTableId)

	if h.conf.DryRun {
		return nil
	}

	_, err := h.ec2s.ReplaceRoute(route)
	return err
}

// discard tables attached to the main table if --ignore-main-table is specified,
// and keep only the table(s) specified with --table/-b (h.conf.RouteTables) if any.
func (h *Hoster) filterRouteTables(tables []*ec2.RouteTable) []*ec2.RouteTable {
	var rt []*ec2.RouteTable
	for _, table := range tables {
		if h.conf.NoMain && isMainTableAssociated(table) {
			continue
		}

		if len(h.conf.RouteTables) == 0 {
			rt = append(rt, table)
			continue
		}

		for _, tbl := range h.conf.RouteTables {
			if table.RouteTableId != nil && *table.RouteTableId == tbl {
				rt = append(rt, table)
				break
			}
		}
	}

	return rt
}

func isMainTableAssociated(table *ec2.RouteTable) bool {
	for _, assoc := range table.Associations {
		if *assoc.Main {
			return true
		}
	}
	return false
}
