package common

import (
	"os"
	"time"
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

type FileLogger struct {
	file *os.File
}
