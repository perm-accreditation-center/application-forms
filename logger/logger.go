package logger

import (
	"application-forms/common"
	"context"
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"
)

type GoogleSheetsLogger struct {
	service     *sheets.Service
	spreadsheet string
	sheetName   string
}

type FileLogger struct {
	file *os.File
}

func NewGoogleSheetsLogger(credentialsPath, spreadsheetID, sheetName string) (*GoogleSheetsLogger, error) {
	ctx := context.Background()

	credBytes, err := os.ReadFile(credentialsPath)
	if err != nil {
		return nil, fmt.Errorf("unable to read credentials file: %v", err)
	}

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

func (l *GoogleSheetsLogger) Log(submission *common.FormSubmission) error {
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
