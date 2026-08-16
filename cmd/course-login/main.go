package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"example.com/course-code-login/internal/courseaccess"
	"example.com/course-code-login/internal/infrai"
)

type server struct {
	access *courseaccess.Service
}

type codeRequest struct {
	LearnerID string `json:"learner_id"`
	Code      string `json:"code"`
}

func main() {
	codes, err := infrai.NewSMSCodes(os.Getenv("INFRAI_API_KEY"))
	if err != nil {
		log.Fatal(err)
	}
	deadline := time.Now().UTC().Add(7 * 24 * time.Hour)
	access := courseaccess.NewService(codes, []courseaccess.Enrollment{{
		LearnerID: "learner-17", CourseID: "go-apis", CourseName: "Go API Design",
		EducatorID: "educator-4", Phone: "+14155550123", Deadline: deadline,
	}})
	s := &server{access: access}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /login/code", s.sendCode)
	mux.HandleFunc("POST /login/verify", s.verifyCode)
	mux.HandleFunc("GET /educators/{id}/report", s.report)
	log.Println("course login listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", mux))
}

func (s *server) sendCode(w http.ResponseWriter, r *http.Request) {
	var input codeRequest
	if err := decode(r, &input); err != nil || input.LearnerID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "learner_id is required"})
		return
	}
	requestID := requestID(r, "send", input.LearnerID)
	if err := s.access.SendCode(r.Context(), input.LearnerID, requestID, time.Now().UTC()); err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "code_sent", "learner_id": input.LearnerID})
}

func (s *server) verifyCode(w http.ResponseWriter, r *http.Request) {
	var input codeRequest
	if err := decode(r, &input); err != nil || input.LearnerID == "" || input.Code == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "learner_id and code are required"})
		return
	}
	requestID := requestID(r, "verify", input.LearnerID)
	decision, err := s.access.VerifyAndOpen(r.Context(), input.LearnerID, input.Code, requestID, time.Now().UTC())
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, decision)
}

func (s *server) report(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.access.Report(r.PathValue("id")))
}

func decode(r *http.Request, dst any) error {
	defer r.Body.Close()
	return json.NewDecoder(http.MaxBytesReader(nil, r.Body, 1<<20)).Decode(dst)
}

func requestID(r *http.Request, action, learnerID string) string {
	if value := strings.TrimSpace(r.Header.Get("X-Request-ID")); value != "" {
		return value
	}
	return fmt.Sprintf("%s-%s-%d", action, learnerID, time.Now().UnixNano())
}

func writeServiceError(w http.ResponseWriter, err error) {
	status := http.StatusBadGateway
	var apiErr *infrai.APIError
	if errors.As(err, &apiErr) && apiErr.StatusCode >= 400 && apiErr.StatusCode < 500 {
		status = apiErr.StatusCode
	}
	if errors.Is(err, courseaccess.ErrDeadlineExpired) {
		status = http.StatusForbidden
	}
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
