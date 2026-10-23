package main

import (
	"encoding/json"
	"fmt"
)

type AppointmentInput struct {
	AppointmentID string `json:"appointment_id"`
	PatientID     string `json:"patient_id"`
	ScheduledAt   string `json:"scheduled_at"`
	ArrivalWindow string `json:"arrival_window"`
}

type AppointmentSnapshot struct {
	AppointmentID string `json:"appointment_id"`
	PatientID     string `json:"patient_id"`
	Schedule      string `json:"schedule"`
	Schema        string `json:"schema"`
	Notification  string `json:"notification"`
}

func decideAppointment(input AppointmentInput, newSchema bool) (AppointmentSnapshot, error) {
	if input.AppointmentID == "" || input.PatientID == "" || input.ScheduledAt == "" {
		return AppointmentSnapshot{}, fmt.Errorf("appointment_id, patient_id, and scheduled_at are required")
	}
	snapshot := AppointmentSnapshot{
		AppointmentID: input.AppointmentID,
		PatientID:     input.PatientID,
		Schedule:      input.ScheduledAt,
		Schema:        "scheduled_at",
		Notification:  "Appointment schedule recorded for care operations.",
	}
	if newSchema && input.ArrivalWindow != "" {
		snapshot.Schedule = input.ArrivalWindow
		snapshot.Schema = "arrival_window"
		snapshot.Notification = "Arrival window recorded; care operations should review the updated handoff."
	}
	return snapshot, nil
}

func encodeSnapshot(snapshot AppointmentSnapshot) ([]byte, error) {
	return json.Marshal(snapshot)
}
