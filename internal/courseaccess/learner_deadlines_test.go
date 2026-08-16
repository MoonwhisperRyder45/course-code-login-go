package courseaccess

import (
	"context"
	"errors"
	"testing"
	"time"
)

type codeStub struct {
	verifyCalls int
}

func (s *codeStub) RequestCode(context.Context, string, string) error { return nil }
func (s *codeStub) VerifyCode(context.Context, string, string, string) error {
	s.verifyCalls++
	return nil
}

func TestVerifyAndOpenHonorsCourseDeadline(t *testing.T) {
	deadline := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name         string
		now          time.Time
		wantStatus   string
		wantErr      error
		wantVerifies int
	}{
		{name: "verified before deadline opens course", now: deadline.Add(-time.Minute), wantStatus: "course_open", wantVerifies: 1},
		{name: "expired enrollment stops before SMS verification", now: deadline.Add(time.Minute), wantStatus: "deadline_expired", wantErr: ErrDeadlineExpired},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			codes := &codeStub{}
			service := NewService(codes, []Enrollment{{
				LearnerID: "learner-17", CourseID: "go-apis", CourseName: "Go API Design",
				EducatorID: "educator-4", Phone: "+14155550123", Deadline: deadline,
			}})
			decision, err := service.VerifyAndOpen(context.Background(), "learner-17", "481205", "verify-17", tt.now)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("error = %v, want %v", err, tt.wantErr)
			}
			if decision.Status != tt.wantStatus {
				t.Fatalf("status = %q, want %q", decision.Status, tt.wantStatus)
			}
			if codes.verifyCalls != tt.wantVerifies {
				t.Fatalf("verify calls = %d, want %d", codes.verifyCalls, tt.wantVerifies)
			}
			report := service.Report("educator-4")
			if tt.wantStatus == "course_open" && report.AccessGranted != 1 {
				t.Fatalf("access granted = %d, want 1", report.AccessGranted)
			}
			if tt.wantStatus == "deadline_expired" && report.DeadlineExpired != 1 {
				t.Fatalf("deadline expired = %d, want 1", report.DeadlineExpired)
			}
		})
	}
}
