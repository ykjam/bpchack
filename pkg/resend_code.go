package pkg

type ResendCodeRequest struct {
	// application trying to use bpc hack, for information purpose only
	Application string `json:"application"`
	// to identify each users request one from another
	Identity      string `json:"identity"`
	ACSRequestId  string `json:"acs_request_id"`
	ACSSessionUrl string `json:"acs_session_url"`
}

type ResendCodeResponse struct {
	Status         HackResponseStatus `json:"status"`
	CurrentAttempt int                `json:"current_attempt,omitempty"`
	TotalAttempts  int                `json:"total_attempts"`
}
