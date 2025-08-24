package helpers

import (
	"context"
	"encoding/json"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

// RabbitPublisher wraps an AMQP channel and queue for publishing messages.
type RabbitPublisher struct {
	conn     *amqp.Connection
	ch       *amqp.Channel
	Queue    string
	Exchange string
}

func NewRabbitPublisher(url, queue string) (*RabbitPublisher, error) {
	conn, err := amqp.Dial(url)
	if err != nil {
		return nil, err
	}
	ch, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	// Declare durable queue
	_, err = ch.QueueDeclare(
		queue,
		true,  // durable
		false, // autoDelete
		false, // exclusive
		false, // noWait
		nil,
	)
	if err != nil {
		_ = ch.Close()
		_ = conn.Close()
		return nil, err
	}
	return &RabbitPublisher{conn: conn, ch: ch, Queue: queue}, nil
}

func (p *RabbitPublisher) Close() {
	if p == nil {
		return
	}
	if p.ch != nil {
		_ = p.ch.Close()
	}
	if p.conn != nil {
		_ = p.conn.Close()
	}
}

// PublishJSON publishes a JSON-encoded message to the default queue.
func (p *RabbitPublisher) PublishJSON(ctx context.Context, body any) error {
	b, err := json.Marshal(body)
	if err != nil {
		return err
	}
	return p.ch.PublishWithContext(ctx,
		"",      // default exchange
		p.Queue, // routing key = queue
		false,   // mandatory
		false,   // immediate
		amqp.Publishing{
			ContentType:  "application/json",
			DeliveryMode: amqp.Persistent,
			Timestamp:    time.Now().UTC(),
			Body:         b,
		},
	)
}
