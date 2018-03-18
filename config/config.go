package config

// CfiConfig is the configuration structucture
type CfiConfig struct {
	// IP is the address we will target routes at. Only mandatory and non guessable argument.
	IP string

	// Hoster (AWS or GCP) can be guessed automaticaly when we run on an instance
	Hoster string

	// Instance name or ID
	Instance string

	// When DryRun is true, we don't really apply changes
	DryRun bool

	// When Quiet is true, we only display errors
	Quiet bool

	// Project (GCP only) identifies the Google Project (guessed on instance)
	Project string

	// Zone is the AWS or GCP zone of the target instance
	Zone string

	// Region is the AWS region
	Region string

	// Ignore tables associated with the main route table
	NoMain bool

	// Interface ID
	Iface string

	// Subnet ID
	Subnet string

	// Target private IP
	TargetIP string

	// AwsAccesKeyID (AWS only) is the acccess key to use (if we don't use an instance profile's role)
	AwsAccesKeyID string

	// AwsSecretKey (AWS only) is the secret key to use (if we don't use an instance profile's role)
	AwsSecretKey string
}
