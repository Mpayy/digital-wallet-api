package queue

import (
	"context"
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"
)

const MidtransWebhookQueue = "midtrans.webhooks"

func declareQueue(ch *amqp.Channel, name string) (amqp.Queue, error) {
	return ch.QueueDeclare(name, true, false, false, false, amqp.Table{
		amqp.QueueTypeArg: amqp.QueueTypeQuorum,
	})
}

//go:generate mockery
//mockery:generate: true
//mockery:filename: ../mocks/mock_publisher.go
type Publisher interface {
	Publish(ctx context.Context, queueName string, body []byte) error
}

type publisherConfig struct {
	ch *amqp.Channel
}

func NewPublisher(ch *amqp.Channel) Publisher {
	return &publisherConfig{
		ch: ch,
	}
}

func (p *publisherConfig) Publish(ctx context.Context, queueName string, body []byte) error {
	if _, err := declareQueue(p.ch, queueName); err != nil {
		return fmt.Errorf("declare queue: %w", err)
	}
	return p.ch.PublishWithContext(ctx, "", queueName, false, false, amqp.Publishing{
		DeliveryMode: amqp.Persistent,
		ContentType:  "application/json",
		Body:         body,
	})
}

//go:generate mockery
//mockery:generate: true
//mockery:filename: ../mocks/mock_consumer.go
type Consumer interface {
	Consume(ctx context.Context, queueName string, handler func(body []byte) error) error
}

type consumerConfig struct {
	ch *amqp.Channel
}

func NewConsumer(ch *amqp.Channel) Consumer {
	return &consumerConfig{
		ch: ch,
	}
}

func (c *consumerConfig) Consume(ctx context.Context, queueName string, handler func(body []byte) error) error {
	if _, err := declareQueue(c.ch, queueName); err != nil {
		return fmt.Errorf("declare queue: %w", err)
	}

	if err := c.ch.Qos(1, 0, false); err != nil {
		return fmt.Errorf("set qos: %w", err)
	}

	msgs, err := c.ch.ConsumeWithContext(ctx, queueName, "", false, false, false, false, nil)
	if err != nil {
		return err
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err() // sinyal shutdown (SIGTERM) -> keluar bersih
		case msg, ok := <-msgs:
			if !ok {
				return fmt.Errorf("rabbitmq delivery channel closed unexpectedly")
			}
			if err := handler(msg.Body); err != nil {
				msg.Nack(false, true) // lihat catatan di bawah soal requeue
			} else {
				msg.Ack(false)
			}
		}
	}
}
