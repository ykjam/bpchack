package response

type PaymentProcessForm struct {
	Info      string `json:"info"`
	ACSUrl    string `json:"acsUrl,omitempty"`
	PaReq     string `json:"paReq,omitempty"`
	TermUrl   string `json:"termUrl,omitempty"`
	ErrorCode int    `json:"errorCode"`
	Redirect  string `json:"redirect,omitempty"`
}

func (p *PaymentProcessForm) IsValid() bool {
	return !(p.ErrorCode != 0 || p.ACSUrl == "" || p.PaReq == "" || p.TermUrl == "")
}
