package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/rabbitmq/amqp091-go"

	"github.com/ohxorud-dev/puri/services/submission/executor"
	"github.com/ohxorud-dev/puri/services/submission/problem"
	"github.com/ohxorud-dev/puri/services/submission/repository"
)

type Consumer struct {
	conn          *amqp091.Connection
	ch            *amqp091.Channel
	repo          *repository.SubmissionRepository
	runner        *executor.DockerRunner
	problemsPath  string
	testcasesPath string

	wg      sync.WaitGroup
	stopped chan struct{}
}

func NewConsumer(url string, repo *repository.SubmissionRepository, runner *executor.DockerRunner, problemsPath, testcasesPath string) (*Consumer, error) {
	conn, err := amqp091.Dial(url)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to RabbitMQ: %w", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to open channel: %w", err)
	}

	if err := ch.Qos(1, 0, false); err != nil {
		ch.Close()
		conn.Close()
		return nil, fmt.Errorf("failed to set QoS: %w", err)
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

	return &Consumer{
		conn:          conn,
		ch:            ch,
		repo:          repo,
		runner:        runner,
		problemsPath:  problemsPath,
		testcasesPath: testcasesPath,
		stopped:       make(chan struct{}),
	}, nil
}

func (c *Consumer) Start() {
	msgs, err := c.ch.Consume(
		"submission_jobs",
		"",
		false, // manual ack
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		log.Printf("[Consumer] failed to start consuming: %v", err)
		close(c.stopped)
		return
	}

	go func() {
		defer close(c.stopped)
		for msg := range msgs {
			c.wg.Add(1)
			c.processJob(msg)
			c.wg.Done()
		}
	}()
}

func (c *Consumer) processJob(msg amqp091.Delivery) {
	var job JobMessage
	if err := json.Unmarshal(msg.Body, &job); err != nil {
		log.Printf("[Consumer] failed to decode job: %v", err)
		msg.Ack(false)
		return
	}

	ctx := context.Background()

	claimed, err := c.repo.ClaimForJudging(ctx, job.SubmissionID)
	if err != nil {
		log.Printf("[Consumer] failed to claim submission %d: %v", job.SubmissionID, err)
		msg.Nack(false, true)
		return
	}
	if !claimed {
		log.Printf("[Consumer] skipping submission %d (already processed or missing)", job.SubmissionID)
		msg.Ack(false)
		return
	}

	log.Printf("[Consumer] judging submission %d (problem %d, lang %s)", job.SubmissionID, job.ProblemID, job.Language)

	// Load test cases
	testCases, err := problem.LoadTestCases(c.testcasesPath, job.ProblemID)
	if err != nil {
		log.Printf("[Consumer] failed to load test cases for problem %d: %v", job.ProblemID, err)
		_ = c.repo.UpdateStatus(ctx, job.SubmissionID, "RUNTIME_ERROR", "failed to load test cases", 0, 0)
		msg.Ack(false)
		return
	}

	meta, err := problem.LoadMetadata(c.problemsPath, job.ProblemID)
	if err != nil {
		log.Printf("[Consumer] failed to load metadata for problem %d: %v", job.ProblemID, err)
		_ = c.repo.UpdateStatus(ctx, job.SubmissionID, "RUNTIME_ERROR", "failed to load problem", 0, 0)
		msg.Ack(false)
		return
	}

	// Run against all test cases
	var maxTimeMs int32
	var maxMemKb int32
	finalStatus := "ACCEPTED"
	var failResult string

	for i, tc := range testCases {
		runCtx, cancel := context.WithTimeout(ctx, parseDuration(meta.TimeLimit)+10*time.Second)
		res := c.runner.Run(runCtx, job.Language, job.SourceCode, tc.Input, meta.TimeLimit, meta.MemoryLimit)
		cancel()

		if res.Error != nil && res.Result == "INTERNAL_ERROR" {
			log.Printf("[Consumer] internal error on test case %d: %v", i+1, res.Error)
			_ = c.repo.UpdateStatus(ctx, job.SubmissionID, "RUNTIME_ERROR", res.Error.Error(), 0, 0)
			msg.Ack(false)
			return
		}

		if res.ExecutionTimeMs > maxTimeMs {
			maxTimeMs = res.ExecutionTimeMs
		}
		if res.MemoryUsageKb > maxMemKb {
			maxMemKb = res.MemoryUsageKb
		}

		if res.Result != "ACCEPTED" {
			finalStatus = res.Result
			failResult = fmt.Sprintf("test case %d: %s", i+1, res.Result)
			break
		}

		if normalizeOutput(res.Output) != normalizeOutput(tc.Output) {
			finalStatus = "WRONG_ANSWER"
			failResult = fmt.Sprintf("test case %d: wrong answer", i+1)
			break
		}
	}

	result := ""
	if finalStatus == "ACCEPTED" {
		result = fmt.Sprintf("all %d test cases passed", len(testCases))
	} else {
		result = failResult
	}

	_ = c.repo.UpdateStatus(ctx, job.SubmissionID, finalStatus, result, maxTimeMs, maxMemKb)
	log.Printf("[Consumer] submission %d: %s (%dms, %dKB)", job.SubmissionID, finalStatus, maxTimeMs, maxMemKb)
	msg.Ack(false)
}

func normalizeOutput(s string) string {
	return strings.TrimSpace(s)
}

func parseDuration(limit string) time.Duration {
	limit = strings.TrimSpace(limit)
	limit = strings.ReplaceAll(limit, "초", "")
	limit = strings.TrimSpace(limit)
	if sec, err := time.ParseDuration(limit + "s"); err == nil {
		return sec
	}
	return 3 * time.Second
}

func (c *Consumer) Shutdown(ctx context.Context) error {
	if c.ch != nil {
		_ = c.ch.Close()
	}

	done := make(chan struct{})
	go func() {
		c.wg.Wait()
		<-c.stopped
		close(done)
	}()

	select {
	case <-done:
	case <-ctx.Done():
		log.Printf("[Consumer] shutdown timeout, in-flight jobs may be interrupted")
	}

	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

func (c *Consumer) Close() {
	if c.ch != nil {
		c.ch.Close()
	}
	if c.conn != nil {
		c.conn.Close()
	}
}
