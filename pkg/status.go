package pkg

type HackResponseStatus string

const (
	HackResponseStatusOk               HackResponseStatus = "ok"
	HackResponseStatusNetworkError     HackResponseStatus = "network-error"
	HackResponseStatusAlreadyProcessed HackResponseStatus = "already-processed"
	HackResponseStatusWrongOTP         HackResponseStatus = "wrong-otp"
	HackResponseStatusOtherError       HackResponseStatus = "other-error"
	HackResponseStatusSpecifyCVC       HackResponseStatus = "specify-cvc"
)