package provider

import (
	"context"
	"errors"
	"log"
	"math/rand"
	"os"
	"strconv"
	"time"
)

type SimulatedSender struct {
	failRate float64
}

func NewSimulatedSender() *SimulatedSender {
	return &SimulatedSender{failRate: simulatedFailRateFromEnv()}
}

func simulatedFailRateFromEnv() float64 {
	rate := 0.8
	if raw := os.Getenv("SIMULATED_FAIL_RATE"); raw != "" {
		if parsed, err := strconv.ParseFloat(raw, 64); err == nil && parsed >= 0 && parsed <= 1 {
			rate = parsed
		}
	}
	return rate
}

func (s *SimulatedSender) Send(ctx context.Context, to, subject, body string) error {
	delay := time.Duration(100+rand.Intn(401)) * time.Millisecond
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(delay):
	}

	if rand.Float64() < s.failRate {
		return errors.New("simulated provider temporary failure")
	}

	log.Printf("[Notification] Sent email to %s for Order #%s. Amount: %s", to, subject, body)
	return nil
}
