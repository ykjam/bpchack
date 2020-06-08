package response

import "strings"

type PaymentProcessForm struct {
	Info      string `json:"info"`
	ACSUrl    string `json:"acsUrl,omitempty"`
	PaReq     string `json:"paReq,omitempty"`
	TermUrl   string `json:"termUrl,omitempty"`
	ErrorCode int    `json:"errorCode"`
	Error     string `json:"error,omitempty"`
	Redirect  string `json:"redirect,omitempty"`
}

func (p *PaymentProcessForm) IsValid() bool {
	return !(p.ErrorCode != 0 || p.ACSUrl == "" || p.PaReq == "" || p.TermUrl == "")
}

func (p *PaymentProcessForm) IsCardError() bool {
	return p.ErrorCode == 1 &&
		(strings.Contains(p.Error, "Payment system") ||
			strings.Contains(p.Error, "Неизвестная платёжная система"))
}

func (p *PaymentProcessForm) IsCVCError() bool {
	return p.ErrorCode == 1 &&
		(strings.Contains(p.Error, "CVC") ||
			strings.Contains(p.Error, "код"))
}
