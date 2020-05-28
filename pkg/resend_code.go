package pkg

import "fmt"

type ResendCodeRequest struct {
	// application trying to use bpc hack, for information purpose only
	Application string `json:"application"`
	// to identify each users request one from another
	Identity      string `json:"identity"`
	ACSRequestId  string `json:"acs_request_id"`
	ACSSessionUrl string `json:"acs_session_url"`
}

type ResendCodeResponse struct {
	Status             HackResponseStatus `json:"status"`
	ResendAttemptsLeft int                `json:"resend_attempts_left"`
}

func (s *ResendCodeResponse) String() string {
	return fmt.Sprintf("ResendCodeResponse {status: %v, attemptsLeft: %d}", s.Status, s.ResendAttemptsLeft)
}
