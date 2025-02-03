package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/go-redis/redis/v8"
)

type QueueService struct {
	client *redis.Client
	ctx    context.Context
}

func NewRedisQueue(addr string) *QueueService {
	rdb := redis.NewClient(&redis.Options{
		Addr: addr,
	})

	return &QueueService{
		client: rdb,
		ctx:    context.Background(),
	}
}

func (qs *QueueService) EnqueueSubmission(submission *FormSubmission) error {
	jsonData, err := json.Marshal(submission)
	if err != nil {
		return fmt.Errorf("failed to marshal submission: %v", err)
	}

	err = qs.client.RPush(qs.ctx, "form_submissions", jsonData).Err()
	if err != nil {
		return fmt.Errorf("failed to enqueue submission: %v", err)
	}

	return nil
}

func processQueue(qs *QueueService) {
	sheetsLogger, err := NewGoogleSheetsLogger(
		"./credentials/credentials.json",
		"1tbuC96BjI1Jude0EiXvS9f_lp-Q3tDAwIfgpTXpGVxA",
		"Sheet1",
	)
	if err != nil {
		log.Fatalf("Failed to initialize sheets logger: %v", err)
	}

	for {
		result, err := qs.client.BLPop(qs.ctx, 0, "form_submissions").Result()
		if err != nil {
			log.Printf("Error polling queue: %v", err)
			continue
		}

		var submission FormSubmission
		if err := json.Unmarshal([]byte(result[1]), &submission); err != nil {
			log.Printf("Error unmarshaling submission: %v", err)
			continue
		}

		err = sheetsLogger.Log(&submission)
		if err != nil {
			submission.Status = "failed"
			log.Printf("Failed to process submission %s: %v", submission.ID, err)
		} else {
			submission.Status = "completed"
		}

		statusKey := fmt.Sprintf("submission_status:%s", submission.ID)
		qs.client.Set(qs.ctx, statusKey, submission.Status, 24*time.Hour)
	}
}
