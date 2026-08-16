package courseaccess

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

type CodeGateway interface {
	RequestCode(context.Context, string, string) error
	VerifyCode(context.Context, string, string, string) error
}

type Enrollment struct {
	LearnerID  string    `json:"learner_id"`
	CourseID   string    `json:"course_id"`
	CourseName string    `json:"course_name"`
	EducatorID string    `json:"educator_id"`
	Phone      string    `json:"-"`
	Deadline   time.Time `json:"deadline"`
}

type Decision struct {
	LearnerID string    `json:"learner_id"`
	CourseID  string    `json:"course_id"`
	Status    string    `json:"status"`
	CheckedAt time.Time `json:"checked_at"`
}

type EducatorReport struct {
	EducatorID      string `json:"educator_id"`
	AccessGranted   int    `json:"access_granted"`
	DeadlineExpired int    `json:"deadline_expired"`
}

type Service struct {
	codes       CodeGateway
	enrollments map[string]Enrollment
	mu          sync.Mutex
	decisions   []Decision
}

func NewService(codes CodeGateway, enrollments []Enrollment) *Service {
	indexed := make(map[string]Enrollment, len(enrollments))
	for _, enrollment := range enrollments {
		indexed[enrollment.LearnerID] = enrollment
	}
	return &Service{codes: codes, enrollments: indexed}
}

func (s *Service) SendCode(ctx context.Context, learnerID, requestID string, now time.Time) error {
	enrollment, ok := s.enrollments[learnerID]
	if !ok {
		return errors.New("learner enrollment not found")
	}
	if now.After(enrollment.Deadline) {
		s.record(Decision{LearnerID: learnerID, CourseID: enrollment.CourseID, Status: "deadline_expired", CheckedAt: now})
		return ErrDeadlineExpired
	}
	return s.codes.RequestCode(ctx, enrollment.Phone, requestID)
}

var ErrDeadlineExpired = errors.New("course deadline has passed")

func (s *Service) VerifyAndOpen(ctx context.Context, learnerID, code, requestID string, now time.Time) (Decision, error) {
	enrollment, ok := s.enrollments[learnerID]
	if !ok {
		return Decision{}, errors.New("learner enrollment not found")
	}
	if now.After(enrollment.Deadline) {
		decision := Decision{LearnerID: learnerID, CourseID: enrollment.CourseID, Status: "deadline_expired", CheckedAt: now}
		s.record(decision)
		return decision, ErrDeadlineExpired
	}
	if err := s.codes.VerifyCode(ctx, enrollment.Phone, code, requestID); err != nil {
		return Decision{}, fmt.Errorf("verify learner phone: %w", err)
	}
	decision := Decision{LearnerID: learnerID, CourseID: enrollment.CourseID, Status: "course_open", CheckedAt: now}
	s.record(decision)
	return decision, nil
}

func (s *Service) Report(educatorID string) EducatorReport {
	learnerIDs := make(map[string]bool)
	for _, enrollment := range s.enrollments {
		if enrollment.EducatorID == educatorID {
			learnerIDs[enrollment.LearnerID] = true
		}
	}
	report := EducatorReport{EducatorID: educatorID}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, decision := range s.decisions {
		if !learnerIDs[decision.LearnerID] {
			continue
		}
		switch decision.Status {
		case "course_open":
			report.AccessGranted++
		case "deadline_expired":
			report.DeadlineExpired++
		}
	}
	return report
}

func (s *Service) record(decision Decision) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.decisions = append(s.decisions, decision)
}
