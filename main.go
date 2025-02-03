package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"
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

type GoogleSheetsLogger struct {
	service     *sheets.Service
	spreadsheet string
	sheetName   string
}

// FormData represents the structure of incoming form data
type FormData struct {
	Data map[string]interface{} `json:"data"`
}

// Logger configuration
type FileLogger struct {
	file *os.File
}

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

func NewGoogleSheetsLogger(credentialsPath, spreadsheetID, sheetName string) (*GoogleSheetsLogger, error) {
	ctx := context.Background()

	// Read credentials file
	credBytes, err := os.ReadFile(credentialsPath)
	if err != nil {
		return nil, fmt.Errorf("unable to read credentials file: %v", err)
	}

	// Configure Google Sheets client
	config, err := google.JWTConfigFromJSON(credBytes, sheets.SpreadsheetsScope)
	if err != nil {
		return nil, fmt.Errorf("unable to parse credentials: %v", err)
	}

	client := config.Client(ctx)
	service, err := sheets.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve Sheets client: %v", err)
	}

	return &GoogleSheetsLogger{
		service:     service,
		spreadsheet: spreadsheetID,
		sheetName:   sheetName,
	}, nil
}

func NewFileLogger(filename string) (*FileLogger, error) {
	// Ensure log directory exists
	logDir := filepath.Dir(filename)
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create log directory: %v", err)
	}

	file, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %v", err)
	}

	return &FileLogger{file: file}, nil
}

func (l *GoogleSheetsLogger) Log(submission *FormSubmission) error {
	// Prepare row data
	rowData := []interface{}{
		submission.ID,
		submission.CreationDate,
		submission.Date,
		submission.Time,
		submission.Department,
		submission.EventType,
		submission.ResponsibleTeacherContact,
		submission.ScheduleCoordinatorContact,
		submission.Comments,
		submission.Group,
		submission.StudentCategory,
		submission.RequiredEquipmentList,
		submission.Discipline,
		submission.PracticalSkills,
		submission.Specialty,
		submission.Stations,
	}

	// Append row to sheet
	valueRange := &sheets.ValueRange{
		Values: [][]interface{}{rowData},
	}

	_, err := l.service.Spreadsheets.Values.Append(
		l.spreadsheet,
		l.sheetName,
		valueRange,
	).ValueInputOption("RAW").Do()

	return err
}

func (l *FileLogger) Close() error {
	return l.file.Close()
}

// Retry configuration
type RetryConfig struct {
	MaxRetries  int
	RetryDelay  time.Duration
	RetryStatus []int
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
