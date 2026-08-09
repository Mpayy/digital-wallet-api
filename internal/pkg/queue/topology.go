package queue

import (
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"
)

type QueueConfig struct {
	Name          string
	DeliveryLimit int32
}

var KnownQueues = []QueueConfig{
	{Name: MidtransWebhookQueue, DeliveryLimit: 5},
	{Name: XenditWebhookQueue, DeliveryLimit: 5},
}

func SetupTopology(ch *amqp.Channel, queues []QueueConfig) error {
	for _, q := range queues {
		if err := declareQueueWithDLX(ch, q); err != nil {
			return fmt.Errorf("setup topology for %s: %w", q.Name, err)
		}
	}
	return nil
}

func declareQueueWithDLX(ch *amqp.Channel, cfg QueueConfig) error {
	dlxName := cfg.Name + ".dlx"
	dlqName := cfg.Name + ".dlq"

	if err := ch.ExchangeDeclare(dlxName, "fanout", true, false, false, false, nil); err != nil {
		return fmt.Errorf("declare dlx exchange: %w", err)
	}
	dlq, err := ch.QueueDeclare(dlqName, true, false, false, false, nil)
	if err != nil {
		return fmt.Errorf("declare dlq: %w", err)
	}
	if err := ch.QueueBind(dlq.Name, "", dlxName, false, nil); err != nil {
		return fmt.Errorf("bind dlq: %w", err)
	}
	_, err = ch.QueueDeclare(cfg.Name, true, false, false, false, amqp.Table{
		amqp.QueueTypeArg:        amqp.QueueTypeQuorum,
		"x-delivery-limit":       cfg.DeliveryLimit,
		"x-dead-letter-exchange": dlxName,
	})
	if err != nil {
		return fmt.Errorf("declare main queue: %w", err)
	}
	return nil
}
