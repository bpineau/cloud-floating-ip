package run

import (
	"fmt"
	"log"

	"github.com/bpineau/cloud-floating-ip/config"
	"github.com/bpineau/cloud-floating-ip/pkg/hoster"
	"github.com/bpineau/cloud-floating-ip/pkg/operation"
)

// Run launchs the effective operations
func Run(conf *config.CfiConfig, op operation.CfiOperation) {

	h, err := hoster.GuessHoster(conf.Hoster)
	if err != nil {
		log.Fatalf("No hoster available: %v", err)
	}

	h.Init(conf)

	if op == operation.CfiPreempt {
		err := h.Preempt()
		if err != nil {
			fmt.Printf("Failed to preempt the IP: %v", err)
		}
	}

	if op == operation.CfiStatus {
		if h.Status() {
			fmt.Println("owner")
		} else {
			fmt.Println("standby")
		}
	}
}
