package pkg

import "fmt"

type ConfirmPaymentRequest struct {
	// application trying to use bpc hack, for information purpose only
	Application string `json:"application"`
	// to identify each users request one from another
	Identity        string `json:"identity"`
	MDOrder         string `json:"md_order"`
	ACSRequestId    string `json:"acs-request-id"`
	ACSSessionUrl   string `json:"acs-session-url,omitempty"`
	OneTimePassword string `json:"one-time-password"`
	TerminateUrl    string `json:"terminate-url"`
}

type ConfirmPaymentResponse struct {
	Status         HackResponseStatus `json:"status"`
	CurrentAttempt int                `json:"current-attempt,omitempty"`
	TotalAttempts  int                `json:"total-attempts,omitempty"`
	FinalUrl       string             `json:"final-url,omitempty"`
}

func (s *ConfirmPaymentResponse) String() string {
	return fmt.Sprintf("ConfirmPaymentResponse {status: %v, cur: %d, tot: %d, finalUrl: %v}",
		s.Status, s.CurrentAttempt, s.TotalAttempts, s.FinalUrl)
}
