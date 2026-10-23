package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
)

type appointmentService struct {
	client *InfraiClient
	bucket string
	flag   string
}

func (s appointmentService) transition(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	defer r.Body.Close()
	var input AppointmentInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	enabled, err := s.client.RolloutEnabled(s.flag)
	if err != nil {
		s.respondInfraiError(w, err)
		return
	}
	snapshot, err := decideAppointment(input, enabled)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	data, err := encodeSnapshot(snapshot)
	if err != nil {
		http.Error(w, "encode snapshot", http.StatusInternalServerError)
		return
	}
	url, err := s.client.PresignPut(s.bucket, snapshotKey(input.AppointmentID), input.AppointmentID)
	if err != nil {
		s.respondInfraiError(w, err)
		return
	}
	if err := s.client.PutSigned(url, data); err != nil {
		http.Error(w, "write appointment snapshot", http.StatusBadGateway)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(snapshot)
}

func (s appointmentService) respondInfraiError(w http.ResponseWriter, err error) {
	var infraiErr *InfraiError
	if errors.As(err, &infraiErr) && infraiErr.Status >= 400 && infraiErr.Status < 500 {
		http.Error(w, infraiErr.Message, infraiErr.Status)
		return
	}
	http.Error(w, "appointment transition unavailable", http.StatusBadGateway)
}

func main() {
	client, err := NewClientFromEnv()
	if err != nil {
		panic(err)
	}
	bucket := envOr("APPOINTMENT_BUCKET", "appointment-rollout-snapshots")
	if err := client.CreateBucket(bucket); err != nil {
		panic(err)
	}
	service := appointmentService{client: client, bucket: bucket, flag: envOr("APPOINTMENT_SCHEMA_FLAG", "appointment-arrival-window")}
	http.HandleFunc("/appointment-transition", service.transition)
	panic(http.ListenAndServe(":8080", nil))
}

func envOr(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
