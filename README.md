# cloud-floating-ip

Implement a floating/virtual IP by configuring cloud provider's routes.

Choose an arbitrary private IP address, and `cloud-floating-ip` will
route traffic for that IP to the AWS or GCP instance of your choice.

## Usage

Choosing the virtual IP: this address must not be already used in the
VPC; it doesn't have to be part of an existing subnet range.

All EC2/GCE instances that may become "primary" (carry the floating IP)
at some point should be allowed by the cloud provider to route traffic
(`SourceDestCheck` (EC2) or `canIpForward` (GCE) must be enabled).

Those instances should be able to accept traffic to the floating IP.
To that effect, we can assign the address to a loopback or a dummy
interface on all instances:

```bash
# we can do that on all instances
ip link add dummy0 type dummy
ip address add 10.200.0.50/32 dev dummy0
ip link set dev dummy0 up
```

To route the floating IP through the current instance:
```bash
# see what would change
cloud-floating-ip -i 10.200.0.50 preempt --dry-run

# apply the changes
cloud-floating-ip -i 10.200.0.50 preempt
```

The IP can be preempted by other instances in the VPC, by using the same
`preempt` command.

To verify the status ("primary" or "standby") of any instance:
```bash
cloud-floating-ip -i 10.200.0.50 status
```

When `cloud-floating-ip` runs on the target instance, most settings (region,
instance id, cloud provider, ...) can be guessed from the instance metadata.

To act on a remote instance, we must be more explicit :
```bash
cloud-floating-ip -o aws -i 10.200.0.50 -t i-0e3f4ac17545ce580 -r eu-west-1 status
cloud-floating-ip -o aws -i 10.200.0.50 -t i-0e3f4ac17545ce580 -r eu-west-1 preempt
````

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
running `cloud-floating-ip` from an AWS or GCE instance.

Those settings can be stored in the `/etc/cloud-floating-ip.yaml`
configuration file. You can also pass them through environments (upper
case, prefixed by `CFI_`).


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
* IPv4 only, for now

