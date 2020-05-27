package pkg

type StartHackRequest struct {
	// application trying to use bpc hack, for information purpose only
	Application string `json:"application"`
	// to identify each users request one from another
	Identity string `json:"identity"`
	// url you received to redirect user to (during https://{crappy_bpc_server}/register.do request)
	PaymentUrl string `json:"payment_url"`
}

type StartHackResponse struct {
	Status        HackResponseStatus `json:"status"`
	MDOrder       string             `json:"md_order,omitempty"`
	RemainingTime int                `json:"remaining_time,omitempty"`
	IsCVCRequired bool               `json:"is_cvc_required,omitempty"`
	AmountInfo    string             `json:"amount_info,omitempty"`
}
