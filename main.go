package main

import (
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type FormSubmission struct {
	ID                         string    `json:"id"`
	CreationDate               time.Time `json:"creation_date"`
	Date                       string    `json:"date"`
	Time                       string    `json:"time"`
	Department                 string    `json:"department"`
	EventType                  string    `json:"event_type"`
	ResponsibleTeacherContact  string    `json:"responsible_teacher_contact"`
	ScheduleCoordinatorContact string    `json:"schedule_coordinator_contact"`
	Comments                   string    `json:"comments"`
	Group                      string    `json:"group"`
	StudentCategory            string    `json:"student_category"`
	RequiredEquipmentList      string    `json:"required_equipment_list"`
	Discipline                 string    `json:"discipline"`
	PracticalSkills            string    `json:"practical_skills"`
	Specialty                  string    `json:"specialty"`
	Stations                   string    `json:"stations"`
	Status                     string    `json:"status"`
}

type FormData struct {
	Data map[string]interface{} `json:"data"`
}

func main() {
	queueService := NewRedisQueue("localhost:6379")

	go processQueue(queueService)

	logger, err := NewFileLogger("logs/form_submissions.log")
	if err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}
	defer logger.Close()

	r := gin.Default()

	r.POST("/submit", func(c *gin.Context) {
		var submission FormSubmission

		if err := c.ShouldBindJSON(&submission); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid submission format"})
			return
		}

		submission.ID = uuid.New().String()
		submission.CreationDate = time.Now()
		submission.Status = "pending"

		err := queueService.EnqueueSubmission(&submission)
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
