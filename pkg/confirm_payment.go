package pkg

type ConfirmPaymentRequest struct {
	// application trying to use bpc hack, for information purpose only
	Application string `json:"application"`
	// to identify each users request one from another
	Identity        string `json:"identity"`
	MDOrder         string `json:"md_order"`
	ACSRequestId    string `json:"acs_request_id"`
	OneTimePassword string `json:"one_time_password"`
	TerminateUrl    string `json:"terminate_url"`
}

type ConfirmPaymentResponse struct {
	Status   HackResponseStatus `json:"status"`
	FinalUrl string             `json:"final_url,omitempty"`
}
