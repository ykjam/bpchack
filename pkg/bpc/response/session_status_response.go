package response

type SessionStatus struct {
	// probably time till order will expire
	RemainingSecs int64  `json:"remainingSecs,omitempty"`
	Redirect      string `json:"redirect,omitempty"`
	// probably some kind of black magick, always zero
	SessionStatus SessionStatusCode `json:"sessionStatus"`
	OrderNumber   string            `json:"orderNumber,omitempty"`
	// too professional, amount and currency in string
	Amount         string `json:"amount,omitempty"`
	Description    string `json:"description,omitempty"`
	BonusAmount    int64  `json:"bonusAmount"`
	SslOnly        bool   `json:"sslOnly"`
	CvcNotRequired bool   `json:"cvcNotRequired"`
	EpinAllowed    bool   `json:"epinAllowed"`
	FeeAllowed     bool   `json:"feeAllowed"`
}

func (s *SessionStatus) IsValid() bool {
	return !(s.RemainingSecs == 0 ||
		s.OrderNumber == "" ||
		s.Amount == "" ||
		s.Description == "")
}

type SessionStatusCode int

const (
	// The only observed status code, if you observe status code which is not zero, please let us know.
	// I think they use some kind of black magick, in order to
	SessionStatusCodeZero SessionStatusCode = 0
)
