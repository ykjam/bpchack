package pkg

type SubmitCardRequest struct {
	// application trying to use bpc hack, for information purpose only
	Application string `json:"application"`
	// to identify each users request one from another
	Identity   string `json:"identity"`
	MDOrder    string `json:"md_order"`
	CardNumber string `json:"card_number"`
	Expiry     string `json:"expiry"`
	NameOnCard string `json:"name_on_card"`
	CVCCode    string `json:"cvc_code,omitempty"`
}

type SubmitCardResponse struct {
	Status        HackResponseStatus `json:"status"`
	ACSRequestId  string             `json:"acs_request_id,omitempty"`
	ACSSessionUrl string             `json:"acs_session_url,omitempty"`
	// number shown in acs form
	ThreeDSecureNumber string `json:"three_d_secure_number,omitempty"`
	TerminateUrl       string `json:"terminate_url,omitempty"`
}
