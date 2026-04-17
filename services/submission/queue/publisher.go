package queue

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/rabbitmq/amqp091-go"
)

type Publisher struct {
	conn *amqp091.Connection
	ch   *amqp091.Channel
}

type JobMessage struct {
	SubmissionID int64  `json:"submission_id"`
	ProblemID    int32  `json:"problem_id"`
	Language     string `json:"language"`
	SourceCode   string `json:"source_code"`
}

func NewPublisher(url string) (*Publisher, error) {
	conn, err := amqp091.Dial(url)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to RabbitMQ: %w", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to open channel: %w", err)
	}

	_, err = ch.QueueDeclare(
		"submission_jobs",
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		ch.Close()
		conn.Close()
		return nil, fmt.Errorf("failed to declare queue: %w", err)
	}

	return &Publisher{conn: conn, ch: ch}, nil
}

func (p *Publisher) Publish(ctx context.Context, job JobMessage) error {
	body, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("failed to marshal job: %w", err)
	}

	return p.ch.PublishWithContext(ctx,
		"",
		"submission_jobs",
		false,
		false,
		amqp091.Publishing{
			ContentType: "application/json",
			Body:        body,
		},
	)
}

func (p *Publisher) Close() {
	if p.ch != nil {
		p.ch.Close()
	}
	if p.conn != nil {
		p.conn.Close()
	}
}
