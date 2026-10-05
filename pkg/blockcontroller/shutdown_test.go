package blockcontroller

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

type shutdownController struct {
	normalDestroyController
	entered chan struct{}
	release chan struct{}
	saved   chan struct{}
}

func (*shutdownController) GetRuntimeStatus() *BlockControllerRuntimeStatus {
	return &BlockControllerRuntimeStatus{ShellProcStatus: Status_Running}
}

func (c *shutdownController) Stop(graceful bool, _ string, _ bool) {
	close(c.entered)
	if !graceful {
		return
	}
	<-c.release
	close(c.saved)
}

func TestShutdownWaitsForControllerPersistence(t *testing.T) {
	id := uuid.NewString()
	c := &shutdownController{entered: make(chan struct{}), release: make(chan struct{}), saved: make(chan struct{})}
	registerController(id, c)
	defer func() { registryLock.Lock(); delete(controllerRegistry, id); registryLock.Unlock() }()
	done := make(chan error, 1)
	go func() { done <- StopAllBlockControllersForShutdown(context.Background()) }()
	<-c.entered
	select {
	case <-done:
		t.Fatal("shutdown returned before controller persistence finished")
	case <-time.After(20 * time.Millisecond):
	}
	close(c.release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	select {
	case <-c.saved:
	default:
		t.Fatal("shutdown missed the final persistence acknowledgement")
	}
}

func TestShutdownControllerTimeoutIsReported(t *testing.T) {
	id := uuid.NewString()
	c := &shutdownController{entered: make(chan struct{}), release: make(chan struct{}), saved: make(chan struct{})}
	registerController(id, c)
	defer func() {
		registryLock.Lock()
		delete(controllerRegistry, id)
		registryLock.Unlock()
		close(c.release)
		<-c.saved
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	if err := StopAllBlockControllersForShutdown(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("unconfirmed controller shutdown must report deadline: %v", err)
	}
}
