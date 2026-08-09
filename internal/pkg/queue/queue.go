package queue

import (
	"context"
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/sirupsen/logrus"
)

const MidtransWebhookQueue = "midtrans.webhooks"
const XenditWebhookQueue = "xendit.webhooks"

//go:generate mockery
//mockery:generate: true
//mockery:filename: ../mocks/mock_publisher.go
type Publisher interface {
	Publish(ctx context.Context, queueName string, body []byte) error
}

type publisherConfig struct {
	ch  *amqp.Channel
	log *logrus.Logger
}

func NewPublisher(ch *amqp.Channel, log *logrus.Logger) Publisher {
	return &publisherConfig{
		ch:  ch,
		log: log,
	}
}

func (p *publisherConfig) Publish(ctx context.Context, queueName string, body []byte) error {
	logger := p.log.WithFields(logrus.Fields{
		"queue_name": queueName,
	})

	err := p.ch.PublishWithContext(ctx, "", queueName, false, false, amqp.Publishing{
		DeliveryMode: amqp.Persistent,
		ContentType:  "application/json",
		Body:         body,
	})

	if err != nil {
		return fmt.Errorf("publish message to %s: %w", queueName, err)
	}

	logger.Debug("message published successfully")
	return nil

}

//go:generate mockery
//mockery:generate: true
//mockery:filename: ../mocks/mock_consumer.go
type Consumer interface {
	Consume(ctx context.Context, queueName string, handler func(body []byte) error) error
}

type consumerConfig struct {
	ch  *amqp.Channel
	log *logrus.Logger
}

func NewConsumer(ch *amqp.Channel, log *logrus.Logger) Consumer {
	return &consumerConfig{
		ch:  ch,
		log: log,
	}
}

func (c *consumerConfig) Consume(ctx context.Context, queueName string, handler func(body []byte) error) error {
	logger := c.log.WithFields(logrus.Fields{
		"queue_name": queueName,
	})

	if err := c.ch.Qos(1, 0, false); err != nil {
		return fmt.Errorf("set qos: %w", err)
	}

	msgs, err := c.ch.ConsumeWithContext(ctx, queueName, "", false, false, false, false, nil)
	if err != nil {
		return err
	}

	logger.Info("rabbitmq consumer started")
	defer logger.Info("rabbitmq consumer stopped")

	for msg := range msgs {
		msgLogger := logger.WithFields(logrus.Fields{
			"msg_id":      msg.MessageId,
			"routing_key": msg.RoutingKey,
		})
		msgLogger.Debug("received message from queue")

		if err := handler(msg.Body); err != nil {
			msgLogger.WithError(err).Error("failed to process message, re-queueing")
			if nackErr := msg.Nack(false, true); nackErr != nil {
				msgLogger.WithError(nackErr).Error("failed to nack message")
			}
		} else {
			msgLogger.Debug("message processed successfully, acking")
			if ackErr := msg.Ack(false); ackErr != nil {
				msgLogger.WithError(ackErr).Error("failed to ack message")
			}
		}
	}

	if err := ctx.Err(); err != nil {
		logger.Info("stopping consumer due to context cancellation")
		return err
	}

	return fmt.Errorf("rabbitmq delivery channel closed unexpectedly")
}
