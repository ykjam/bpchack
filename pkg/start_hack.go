package pkg

type StartHackRequest struct {
	// application trying to use bpc hack, for information purpose only
	Application string `json:"app"`
	// to identify each users request one from another
	Identity string `json:"id"`
	// url you received to redirect user to (during https://{crappy_bpc_server}/register.do request)
	PaymentUrl string `json:"url"`
}

type StartHackResponse struct {
	Status        HackResponseStatus `json:"status"`
	MDOrder       string             `json:"md-order,omitempty"`
	RemainingTime int64                `json:"remaining-time,omitempty"`
	ExpirationTs  int64              `json:"expiration-ts,omitempty"`
	IsCVCRequired bool               `json:"is-cvc-required,omitempty"`
	AmountInfo    string             `json:"amount-info,omitempty"`
}
