package pkg

import "fmt"

type ResendCodeRequest struct {
	// application trying to use bpc hack, for information purpose only
	Application string `json:"app"`
	// to identify each users request one from another
	Identity      string `json:"id"`
	ACSRequestId  string `json:"acs-request-id"`
	ACSSessionUrl string `json:"acs-session-url"`
}

type ResendCodeResponse struct {
	Status             HackResponseStatus `json:"status"`
	ResendAttemptsLeft int                `json:"resend-attempts-left"`
}

func (s *ResendCodeResponse) String() string {
	return fmt.Sprintf("ResendCodeResponse {status: %v, attemptsLeft: %d}", s.Status, s.ResendAttemptsLeft)
}
