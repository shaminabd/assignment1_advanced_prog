package provider

import (
	"fmt"
	"os"
	"strings"
)

func NewEmailSenderFromEnv() (EmailSender, error) {
	mode := strings.ToUpper(strings.TrimSpace(os.Getenv("PROVIDER_MODE")))
	if mode == "" {
		mode = "SIMULATED"
	}

	switch mode {
	case "SIMULATED":
		return NewSimulatedSender(), nil
	case "REAL":
		return NewSMTPSender()
	default:
		return nil, fmt.Errorf("unsupported PROVIDER_MODE %q", mode)
	}
}
