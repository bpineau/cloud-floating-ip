package run

import (
	"fmt"

	"github.com/bpineau/cloud-floating-ip/config"
	"github.com/bpineau/cloud-floating-ip/pkg/hoster"
	"github.com/bpineau/cloud-floating-ip/pkg/log/console"
	"github.com/bpineau/cloud-floating-ip/pkg/operation"
)

// Run launchs the effective operations
func Run(conf *config.CfiConfig, op operation.CfiOperation) {
	var err error

	log := &console.Logger{Quiet: conf.Quiet}

	h, err := hoster.GuessHoster(conf.Hoster)
	if err != nil {
		log.Fatalf("Can't guess hoster, please specify '-o' option: %v", err)
	}

	h.Init(conf, log)

	switch op {
	case operation.CfiPreempt:
		err = h.Preempt()
	case operation.CfiDestroy:
		err = h.Destroy()
	case operation.CfiStatus:
		if h.Status() {
			fmt.Println("primary")
		} else {
			fmt.Println("standby")
		}
	}

	if err != nil {
		log.Fatal(err)
	}
}
