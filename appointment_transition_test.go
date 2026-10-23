package main

import "testing"

func TestDecideAppointmentUsesFlaggedArrivalWindow(t *testing.T) {
	input := AppointmentInput{AppointmentID: "apt-42", PatientID: "patient-9", ScheduledAt: "2026-10-04T09:00:00Z", ArrivalWindow: "2026-10-04T08:40:00Z/09:00:00Z"}

	got, err := decideAppointment(input, true)
	if err != nil {
		t.Fatal(err)
	}
	if got.Schema != "arrival_window" || got.Schedule != input.ArrivalWindow {
		t.Fatalf("expected flagged schema and arrival window, got %+v", got)
	}

	rollback, err := decideAppointment(input, false)
	if err != nil {
		t.Fatal(err)
	}
	if rollback.Schema != "scheduled_at" || rollback.Schedule != input.ScheduledAt {
		t.Fatalf("expected rollback to scheduled_at, got %+v", rollback)
	}
}
