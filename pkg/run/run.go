package run

import (
	"fmt"

	"github.com/bpineau/cloud-floating-ip/config"
	"github.com/bpineau/cloud-floating-ip/pkg/hoster"
	"github.com/bpineau/cloud-floating-ip/pkg/operation"
)

// Run launchs the effective operations
func Run(conf *config.CfiConfig, op operation.CfiOperation) {
	_ = hoster.Hosters // noop
	fmt.Printf("from run.go. Hoster: '%s'. Current op: %d\n", conf.Hoster, op)
}
