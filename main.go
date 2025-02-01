package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/gin-gonic/gin"
)

// FormData represents the structure of incoming form data
type FormData struct {
	Data map[string]interface{} `json:"data"`
}

// Logger configuration
type FileLogger struct {
	file *os.File
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

func (l *FileLogger) Log(data interface{}) error {
	logEntry := struct {
		Timestamp time.Time   `json:"timestamp"`
		Data      interface{} `json:"data"`
	}{
		Timestamp: time.Now(),
		Data:      data,
	}

	jsonData, err := json.Marshal(logEntry)
	if err != nil {
		return fmt.Errorf("failed to marshal log entry: %v", err)
	}

	if _, err := l.file.Write(append(jsonData, '\n')); err != nil {
		return fmt.Errorf("failed to write to log file: %v", err)
	}

	return nil
}

func (l *FileLogger) Close() error {
	return l.file.Close()
}

// IPv6 CIDR validator for Yandex Forms network
func isYandexIPv6(ip net.IP) bool {
	yandexNetwork := net.IPNet{
		IP:   net.ParseIP("2a02:6b8:c00::"),
		Mask: net.CIDRMask(40, 128),
	}
	return yandexNetwork.Contains(ip)
}

// Middleware to check IPv6 address
func ipv6Check() gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := net.ParseIP(c.ClientIP())
		if ip == nil || ip.To4() != nil || !isYandexIPv6(ip) {
			c.JSON(http.StatusForbidden, gin.H{
				"error": "Access denied: Invalid IPv6 address or not from Yandex network",
			})
			c.Abort()
			return
		}
		c.Next()
	}
}

// Retry configuration
type RetryConfig struct {
	MaxRetries  int
	RetryDelay  time.Duration
	RetryStatus []int
}

func shouldRetry(statusCode int, config RetryConfig) bool {
	for _, code := range config.RetryStatus {
		if statusCode == code {
			return true
		}
	}
	return false
}

func main() {
	// Initialize logger
	logger, err := NewFileLogger("logs/form_submissions.log")
	if err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}
	defer logger.Close()

	// Initialize Gin
	r := gin.Default()

	// Configure retry settings
	retryConfig := RetryConfig{
		MaxRetries:  3,
		RetryDelay:  time.Second * 2,
		RetryStatus: []int{500, 502, 503, 504, 404},
	}

	// Form submission handler
	r.POST("/submit", ipv6Check(), func(c *gin.Context) {
		var formData FormData

		// Read the request body
		bodyBytes, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to read request body"})
			return
		}
		// Restore the request body for further processing
		c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

		// Parse JSON data
		if err := c.ShouldBindJSON(&formData); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON format"})
			return
		}

		// Log the submission
		if err := logger.Log(formData.Data); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to log submission"})
			return
		}

		// Process submission with retry logic
		var processErr error
		for retry := 0; retry <= retryConfig.MaxRetries; retry++ {
			if retry > 0 {
				time.Sleep(retryConfig.RetryDelay)
			}

			// Process the submission (example)
			resp, err := processSubmission(formData.Data)
			if err != nil {
				processErr = err
				continue
			}

			if shouldRetry(resp.StatusCode, retryConfig) {
				processErr = fmt.Errorf("received status code: %d", resp.StatusCode)
				continue
			}

			// Success
			c.JSON(http.StatusOK, gin.H{
				"status":  "success",
				"message": "Form submission processed successfully",
			})
			return
		}

		// If we get here, all retries failed
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Failed to process submission after %d retries: %v",
				retryConfig.MaxRetries, processErr),
		})
	})

	// Start server with IPv6 support
	log.Fatal(r.Run("[::]:8081"))
}

// Mock function to simulate processing submission
func processSubmission(data interface{}) (*http.Response, error) {
	// In a real implementation, this would send the data to your backend service
	return &http.Response{StatusCode: http.StatusOK}, nil
}
