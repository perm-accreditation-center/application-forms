package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"
)

type FormSubmission struct {
	ID                         string `json:"id"`
	CreationDate               string `json:"creation_date"`
	Date                       string `json:"date"`
	Time                       string `json:"time"`
	Department                 string `json:"department"`
	EventType                  string `json:"event_type"`
	ResponsibleTeacherContact  string `json:"responsible_teacher_contact"`
	ScheduleCoordinatorContact string `json:"schedule_coordinator_contact"`
	Comments                   string `json:"comments"`
	Group                      string `json:"group"`
	StudentCategory            string `json:"student_category"`
	RequiredEquipmentList      string `json:"required_equipment_list"`
	Discipline                 string `json:"discipline"`
	PracticalSkills            string `json:"practical_skills"`
	Specialty                  string `json:"specialty"`
	Stations                   string `json:"stations"`
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
	sheetsLogger, err := NewGoogleSheetsLogger(
		"./credentials/credentials.json",
		"1tbuC96BjI1Jude0EiXvS9f_lp-Q3tDAwIfgpTXpGVxA",
		"Sheet1",
	)
	if err != nil {
		log.Fatalf("Failed to initialize Google Sheets logger: %v", err)
	}

	logger, err := NewFileLogger("logs/form_submissions.log")
	if err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}
	defer logger.Close()

	retryConfig := RetryConfig{
		MaxRetries:  3,
		RetryDelay:  time.Second * 2,
		RetryStatus: []int{500, 502, 503, 504, 404},
	}

	r := gin.Default()

	r.POST("/submit", func(c *gin.Context) {
		var submission FormSubmission

		// Parse JSON data
		if err := c.ShouldBindJSON(&submission); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid submission format"})
			return
		}

		// Generate unique ID
		submission.ID = uuid.New().String()
		submission.CreationDate = time.Now().Format(time.RFC3339)

		// Retry logging to Google Sheets
		var logErr error
		for retry := 0; retry <= retryConfig.MaxRetries; retry++ {
			if retry > 0 {
				time.Sleep(retryConfig.RetryDelay)
			}

			logErr = sheetsLogger.Log(&submission)
			if logErr == nil {
				c.JSON(http.StatusOK, gin.H{
					"status":  "success",
					"message": "Form submission processed successfully",
					"id":      submission.ID,
				})
				return
			}
		}

		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Failed to log submission after %d retries: %v",
				retryConfig.MaxRetries, logErr),
		})
	})

	log.Fatal(r.Run(":8081"))

}
