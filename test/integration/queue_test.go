//go:build integration

package integration_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBasicRoundtrip(t *testing.T) {
	conn, queueName := setupRabbitMQ(t)
	publiser := setupPublish(t, conn)
	consumer := setupConsumer(t, conn)

	payload := []byte(`{"test":"queue"}`)

	var wg sync.WaitGroup
	results := make(chan []byte, 1)
	errCh := make(chan error, 1)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	wg.Add(1)
	go func() {
		defer wg.Done()
		err := consumer.Consume(ctx, queueName, func(body []byte) error {
			results <- body
			return nil
		})
		if err != nil && !errors.Is(err, context.Canceled) {
			select {
			case errCh <- err:
			default:
			}
		}
	}()

	err := publiser.Publish(ctx, queueName, payload)
	require.NoError(t, err)

	select {
	case receivedBody := <-results:
		assert.Equal(t, payload, receivedBody, "the payload received must be exactly the same")
	case err := <-errCh:
		require.NoError(t, err, "an error occurred while consuming the queue")
	case <-time.After(5 * time.Second):
		t.Fatal("test timeout - no message received from queue")
	}

	cancel()
	wg.Wait()
}

func TestAckOnSuccess(t *testing.T) {
	conn, queueName := setupRabbitMQ(t)
	publiser := setupPublish(t, conn)
	consumer := setupConsumer(t, conn)

	payload := []byte(`{"test":"ack-success"}`)

	var wg sync.WaitGroup
	var proprocessedCount atomic.Int32

	firstReceiveCh := make(chan struct{}, 1)
	errCh := make(chan error, 1)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	wg.Add(1)
	go func() {
		defer wg.Done()
		err := consumer.Consume(ctx, queueName, func(body []byte) error {
			proprocessedCount.Add(1)
			select {
			case firstReceiveCh <- struct{}{}:
			default:
			}
			return nil
		})
		if err != nil && !errors.Is(err, context.Canceled) {
			select {
			case errCh <- err:
			default:
			}
		}
	}()

	err := publiser.Publish(ctx, queueName, payload)
	require.NoError(t, err)

	select {
	case <-firstReceiveCh:
	case err := <-errCh:
		require.NoError(t, err, "an error occurred while consuming the queue")
	case <-time.After(5 * time.Second):
		t.Fatal("test timeout - no message received from queue")
	}

	time.Sleep(1500 * time.Millisecond)

	assert.Equal(t, int32(1), proprocessedCount.Load(), "Successfully processed messages must be ACKed and NOT retransmitted by RabbitMQ.")

	cancel()
	wg.Wait()
}

func TestConsumer_RequeueOnFailure(t *testing.T) {
	conn, queueName := setupRabbitMQ(t)
	publiser := setupPublish(t, conn)
	consumer := setupConsumer(t, conn)

	payload := []byte(`{"test":"requeue"}`)

	var wg sync.WaitGroup
	var attempts atomic.Int32

	doneCh := make(chan struct{}, 1)
	errCh := make(chan error, 1)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	wg.Add(1)
	go func() {
		defer wg.Done()
		err := consumer.Consume(ctx, queueName, func(body []byte) error {
			count := attempts.Add(1)
			switch count {
			case 1:
				return errors.New("simulated temporary error")
			default:
				select {
				case doneCh <- struct{}{}:
				default:
				}

				return nil
			}
		})
		if err != nil && !errors.Is(err, context.Canceled) {
			select {
			case errCh <- err:
			default:
			}
		}
	}()

	err := publiser.Publish(ctx, queueName, payload)
	require.NoError(t, err)

	select {
	case <-doneCh:
	case err := <-errCh:
		t.Fatalf("Consumer error: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("test timeout - message was not requeued or processed 2nd time")
	}

	assert.Equal(t, int32(2), attempts.Load(), "Handler must be executed exactly 2 times (1x failure, 1x success after requeue)")

	cancel()
	wg.Wait()
}

func TestConsumer_GracefulShutdown(t *testing.T) {
	conn, queueName := setupRabbitMQ(t)
	consumer := setupConsumer(t, conn)

	consumeErrCh := make(chan error, 1)
	startedCh := make(chan struct{})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		close(startedCh)
		err := consumer.Consume(ctx, queueName, func(body []byte) error {
			return nil
		})
		consumeErrCh <- err
	}()

	<-startedCh

	time.Sleep(100 * time.Millisecond)

	cancel()

	select {
	case err := <-consumeErrCh:
		assert.Error(t, err)
		assert.ErrorIs(t, err, context.Canceled)
	case <-time.After(3 * time.Second):
		t.Fatal("Timeout: Consumer did not stop gracefully when context was canceled")
	}
}
