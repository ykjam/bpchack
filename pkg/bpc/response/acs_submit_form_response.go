package response

type ACSSubmitForm struct {
	ACSSessionUrl      string
	ACSRequestId       string // need to extract it from url of redirected page
	ThreeDSecureNumber string // need to extract from html of redirected page
}
