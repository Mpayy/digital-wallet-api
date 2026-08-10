package config

import (
	"crypto/tls"
	"fmt"
	"strings"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

func NewRabbitMQ(config *viper.Viper, log *logrus.Logger) (*amqp.Channel, func(), error) {
	rabbitmqURL := config.GetString("RABBITMQ_URL")
	enableTLS := config.GetBool("RABBITMQ_TLS_ENABLED") // Sesuaikan key env-nya

	var tlsConfig *tls.Config

	// Jika TLS diaktifkan via .env ATAU URL menggunakan CloudAMQP (amqps://)
	if enableTLS || strings.HasPrefix(rabbitmqURL, "amqps://") {
		tlsConfig = &tls.Config{
			MinVersion: tls.VersionTLS12,
		}
	}

	conn, err := amqp.DialConfig(rabbitmqURL, amqp.Config{
		TLSClientConfig: tlsConfig,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("connect to rabbitmq: %w", err)
	}

	log.Info("Connected to RabbitMQ successfully")

	ch, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		return nil, nil, fmt.Errorf("open rabbitmq channel: %w", err)
	}

	log.Info("Opened RabbitMQ channel successfully")

	cleanup := func() {
		if err := ch.Close(); err != nil {
			log.Errorf("failed to close rabbitmq channel: %v", err)
		}
		log.Info("RabbitMQ channel closed")
		if err := conn.Close(); err != nil {
			log.Errorf("failed to close rabbitmq connection: %v", err)
		}
		log.Info("RabbitMQ connection closed")
	}

	return ch, cleanup, nil
}
