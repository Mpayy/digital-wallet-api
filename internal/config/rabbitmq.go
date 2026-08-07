package config

import (
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/spf13/viper"
)

func NewRabbitMQ(config *viper.Viper) (*amqp.Channel, error) {
	conn, err := amqp.Dial(config.GetString("RABBITMQ_URL"))
	if err != nil {
		return nil, fmt.Errorf("connect to rabbitmq: %w", err)
	}
	ch, err := conn.Channel()
	if err != nil {
		return nil, fmt.Errorf("open rabbitmq channel: %w", err)
	}
	return ch, nil
}
