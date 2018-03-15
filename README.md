# cloud-floating-ip

Implement a floating/virtual IP by modifying GCP or AWS routes.

## Usage

All EC2/GCE instances that may carry the floating IP (become "primary")
should be allowed to route traffic: `SourceDestCheck` (EC2) or `canIpForward`
(GCE) must be enabled.

Those instances should accept the traffic to the floating IP, which may
be assigned to a loopback or a dummy interface on all instances:

```bash
ip link add dummy0 type dummy
ip address add 10.200.0.50/32 dev dummy0
```

To route the floating IP to the current instance (becomes "primary"):
```bash
# see what would change
cloud-floating-ip -i 10.200.0.50 preempt --dry-run

# apply the changes
cloud-floating-ip -i 10.200.0.50 preempt
```

The IP can be preempted by other instances in the VP, by using the same
`preempt` command.

To verify the status ("primary" or "standby") of any instance:
```bash
cloud-floating-ip -i 10.200.0.50 status
```

To store the configuration (and get rid of repetitive `-i ...` arguments):
```bash
cat<<EOF > /etc/cloud-floating-ip.yaml
ip: 10.200.0.50
quiet: true
EOF
```

## Options

The `ip` argument is mandatory. Other settings can be collected from
instance's metadata (and instance profile or service account) when
running from an AWS or GCE instance.

Those settings can be stored in the `/etc/cloud-floating-ip.yaml`
configuration file. Or pass them through environments (upper case,
prefixed by `CFI_`).


```
Usage:
  cloud-floating-ip [flags]
  cloud-floating-ip [command]

Available Commands:
  destroy     Delete the routes managed by cloud-floating-ip
  help        Help about any command
  preempt     Preempt an IP address and route it to the instance
  status      Display the status of the instance (owner or standby)

Flags:
  -c, --config string              config file (default is /etc/cloud-floating-ip.yaml)
  -i, --ip string                  IP address
  -d, --dry-run                    dry-run mode
  -q, --quiet                      quiet mode
  -h, --help                       help for cloud-floating-ip
  -o, --hoster string              hosting provider (aws or gce)
  -t, --instance string            instance name
  -m, --ignore-main-table          (AWS) ignore routes in main table
  -a, --aws-access-key-id string   (AWS) access key Id
  -k, --aws-secret-key string      (AWS) secret key
  -r, --region string              (AWS) region name
  -p, --project string             (GCP) project id
  -z, --zone string                (GCP) zone name
```

## Required privileges

On EC2, the account running `cloud-floating-ip` must have the following rights:
```
ec2:DescribeInstances
ec2:DescribeRouteTables
ec2:CreateRoute
ec2:ReplaceRoute
ec2:DeleteRoute
```

On GCE:
```
compute.instances.get
compute.routes.get
compute.routes.create
compute.routes.delete
container.operations.get
container.operations.list
```

## Limitations

* `cloud-floating-ip` does not support instances with multiple interfaces in the VPC yet.
* On GCE, `cloud-floating-ip` won't remove already created, pre-existing routes with a custom name

