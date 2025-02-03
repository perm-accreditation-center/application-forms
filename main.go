package main

import (
	"log"
	"net/http"
	"time"

	"application-forms/common"
	"application-forms/logger"
	"application-forms/queue"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func main() {
	queueService := queue.NewRedisQueue("127.0.0.1:6381")

	go queue.ProcessQueue(queueService)

	logger, err := logger.NewFileLogger("logs/form_submissions.log")
	if err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}
	defer logger.Close()

	r := gin.Default()

	r.POST("/submit", func(c *gin.Context) {
		var submission common.FormSubmission

		if err := c.ShouldBindJSON(&submission); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid submission format"})
			return
		}

		submission.ID = uuid.New().String()
		submission.CreationDate = time.Now()
		submission.Status = "pending"

		queueSubmission := common.FormSubmission(submission)
		err := queueService.EnqueueSubmission(&queueSubmission)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"status": "error",
				"error":  "Failed to queue submission",
			})
			return
		}

		c.JSON(http.StatusAccepted, gin.H{
			"status":  "success",
			"message": "Form submission queued successfully",
			"id":      submission.ID,
		})
	})

	log.Fatal(r.Run(":8081"))
}
