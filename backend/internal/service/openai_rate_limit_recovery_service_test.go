//go:build unit

package service

import (
	"context"
	"testing"
	"time"
)

func TestOpenAIRateLimitRecoveryService_StopCancelsActiveRunContext(t *testing.T) {
	t.Parallel()

	svc := NewOpenAIRateLimitRecoveryService(nil, nil, nil, nil)

	roundDone := make(chan struct{})
	svc.wg.Add(1)
	go func() {
		defer svc.wg.Done()
		ctx, cancel := context.WithTimeout(svc.runCtx, time.Minute)
		defer cancel()
		<-ctx.Done()
		close(roundDone)
	}()

	stopped := make(chan struct{})
	go func() {
		svc.Stop()
		close(stopped)
	}()

	select {
	case <-roundDone:
	case <-time.After(2 * time.Second):
		t.Fatal("active recovery round should be canceled when service stops")
	}

	select {
	case <-stopped:
	case <-time.After(2 * time.Second):
		t.Fatal("Stop should return promptly after canceling the active recovery round")
	}
}
