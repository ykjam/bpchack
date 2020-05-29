package pkg

import "fmt"

type SubmitCardRequest struct {
	// application trying to use bpc hack, for information purpose only
	Application string `json:"app"`
	// to identify each users request one from another
	Identity   string `json:"id"`
	MDOrder    string `json:"md-order"`
	CardNumber string `json:"card-number"`
	Expiry     string `json:"card-expiry"`
	NameOnCard string `json:"name-on-card"`
	CVCCode    string `json:"card-cvc,omitempty"`
}

type SubmitCardResponse struct {
	Status        HackResponseStatus `json:"status"`
	ACSRequestId  string             `json:"acs-request-id,omitempty"`
	ACSSessionUrl string             `json:"acs-session-url,omitempty"`
	// number shown in acs form
	ThreeDSecureNumber string `json:"three-d-secure-number,omitempty"`
	ResendAttemptsLeft int    `json:"resend-attempts-left,omitempty"`
	TerminateUrl       string `json:"terminate-url,omitempty"`
}

func (s SubmitCardResponse) String() string {
	return fmt.Sprintf("SubmitCardResponse {status: %v, reqId: %v, acsUrl: %v, 3ds-num: %v, attLeft: %d, termUrl: %v}",
		s.Status, s.ACSRequestId, s.ACSSessionUrl, s.ThreeDSecureNumber, s.ResendAttemptsLeft, s.TerminateUrl)
}
