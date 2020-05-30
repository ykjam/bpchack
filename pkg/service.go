package pkg

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"

	"ykjam/bpchack/pkg/bpc/response"
)

type Service interface {
	Step1StartHack(ctx context.Context, req StartHackRequest) (StartHackResponse, error)
	Step2SubmitCard(ctx context.Context, req SubmitCardRequest) (SubmitCardResponse, error)
	Step3ResendCode(ctx context.Context, req ResendCodeRequest) (ResendCodeResponse, error)
	Step4ConfirmPayment(ctx context.Context, req ConfirmPaymentRequest) (ConfirmPaymentResponse, error)
}

type service struct {
	timeout    time.Duration
	baseMpiUrl string
}

// <div id="tipContainer" class="tipContainer"><span class="tip">One-time password will be sent to number {3DSECURE}</span></div>
const ThreeDSecurePhoneTipBegin = `<div id="tipContainer" class="tipContainer"><span class="tip">One-time password will be sent to number `
const ThreeDSecurePhoneTipEnd = `</span></div>`

// <a id="resendPasswordLink" href="#" title="{3DSECURE} password send attempt(s) left" onclick="jsf.util.chain
const ThreeDSecurePasswordAttemptsBegin = `<a id="resendPasswordLink" href="#" title="`
const ThreeDSecurePasswordAttemptsEnd = ` password send attempt(s) left" onclick="jsf.util.chain`

// <div id="errorContainer" class="errorContainer"><ul><li class="errorMessage">	Wrong password typed attempt {1} of {3} </li></ul></div>
const ThreeDSecureWrongPasswordAttemptBegin = `<div id="errorContainer" class="errorContainer"><ul><li class="errorMessage">	Wrong password typed attempt `
const ThreeDSecureWrongPasswordAttemptMiddle = ` of `
const ThreeDSecureWrongPasswordAttemptEnd = ` </li></ul></div>`
const ThreeDSecureWrongPasswordFinal = `<span class="operationCancelledMessage">Operation cancelled</span>`

// <input type="hidden" name="PaRes" value="{CODE}" />
const ThreeDSecurePaymentResponseBegin = `<input type="hidden" name="PaRes" value="`
const ThreeDSecurePaymentResponseEnd = `" />`

var ErrWrongPasswordOperationCancelled = errors.New("wrong password, operation cancelled")

func (s *service) generateClient() *http.Client {
	return &http.Client{
		Timeout: s.timeout,
	}
}

func (s *service) getSessionUrl() string {
	return fmt.Sprintf("%s/getSessionStatus.do", s.baseMpiUrl)
}

func (s *service) getProcessFormUrl() string {
	return fmt.Sprintf("%s/processform.do", s.baseMpiUrl)
}

func (s *service) Step1StartHack(ctx context.Context, req StartHackRequest) (resp StartHackResponse, err error) {
	clog := log.WithFields(log.Fields{
		"app":       req.Application,
		"id":        req.Identity,
		"operation": "Step 1. Start Hack",
	})
	clog.Info("Processing")
	resp.Status = HackResponseStatusOtherError
	// parse payment url
	var paymentUrl *url.URL
	paymentUrl, err = url.Parse(req.PaymentUrl)
	if err != nil {
		eMsg := "error parsing payment url"
		clog.WithError(err).Error(eMsg)
		err = errors.Wrap(err, eMsg)
		return
	}
	mdOrder := paymentUrl.Query().Get("mdOrder")
	resp.MDOrder = mdOrder
	// check session status
	client := s.generateClient()
	form := url.Values{}
	form.Add("MDORDER", mdOrder)
	var res *http.Response
	var r *http.Request
	var data []byte

	r, err = http.NewRequestWithContext(ctx, http.MethodPost, s.getSessionUrl(), strings.NewReader(form.Encode()))
	if err != nil {
		eMsg := "error creating http request"
		clog.WithError(err).Error(eMsg)
		err = errors.Wrap(err, eMsg)
		return
	}
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	res, err = client.Do(r)
	if err != nil {
		eMsg := "error making http request"
		clog.WithError(err).Error(eMsg)
		err = errors.Wrap(err, eMsg)
		resp.Status = HackResponseStatusNetworkError
		return
	}
	if res.StatusCode != http.StatusOK {
		eMsg := fmt.Sprintf("invalid http status code: %d", res.StatusCode)
		clog.WithError(err).Error(eMsg)
		err = errors.Wrap(err, eMsg)
		resp.Status = HackResponseStatusOtherError
		return
	}
	data, err = ioutil.ReadAll(res.Body)
	defer func() {
		err = res.Body.Close()
		if err != nil {
			clog.WithError(err).Error("error in response.Body.Close")
		}
	}()

	rawResponse := string(data)
	clog.WithField("raw", rawResponse).Info("Response received")
	if err != nil {
		eMsg := "error reading http response"
		clog.WithError(err).Error(eMsg)
		err = errors.Wrap(err, eMsg)
		return
	}
	var bpcResponse response.SessionStatus
	err = json.Unmarshal(data, &bpcResponse)
	if err != nil {
		eMsg := "error parsing json response"
		clog.WithError(err).Error(eMsg)
		err = errors.Wrap(err, eMsg)
		return
	}
	if !bpcResponse.IsValid() {
		eMsg := "session expired or already processed"
		clog.WithError(err).Error(eMsg)
		err = errors.Wrap(err, eMsg)
		resp.Status = HackResponseStatusAlreadyProcessed
		return
	}
	// response is valid
	resp.Status = HackResponseStatusOk
	resp.MDOrder = mdOrder
	resp.ExpirationTs = bpcResponse.RemainingSecs + time.Now().Unix()
	resp.IsCVCRequired = !bpcResponse.CvcNotRequired
	resp.AmountInfo = bpcResponse.Amount
	return
}

func (s *service) Step2SubmitCard(ctx context.Context, req SubmitCardRequest) (resp SubmitCardResponse, err error) {
	clog := log.WithFields(log.Fields{
		"app":       req.Application,
		"id":        req.Identity,
		"operation": "Step 2. Submit Card",
	})
	clog.Info("Processing")
	resp.Status = HackResponseStatusOtherError

	// submit card
	var bpcResponsePart1 response.PaymentProcessForm
	bpcResponsePart1, err = s.step2part1SubmitCard(ctx, clog, req)
	if err != nil {
		eMsg := "error in part 1"
		clog.WithError(err).Error(eMsg)
		err = errors.Wrap(err, eMsg)
		resp.Status = HackResponseStatusOtherError
		return
	}
	if !bpcResponsePart1.IsValid() {
		eMsg := "invalid bpc response"
		clog.WithError(err).Error(eMsg)
		err = errors.Wrap(err, eMsg)
		if bpcResponsePart1.ErrorCode == 1 {
			resp.Status = HackResponseStatusSpecifyCVC
		} else {
			resp.Status = HackResponseStatusOtherError
		}
		return
	}
	resp.TerminateUrl = bpcResponsePart1.TermUrl

	clog.Info("Submitting ACS Form")
	var bpcResponsePart2 response.ACSSubmitForm
	bpcResponsePart2, err = s.step2part2SubmitACS(ctx, clog, req.MDOrder,
		bpcResponsePart1.PaReq, bpcResponsePart1.ACSUrl, bpcResponsePart1.TermUrl)
	if err != nil {
		eMsg := "error in part 2"
		clog.WithError(err).Error(eMsg)
		err = errors.Wrap(err, eMsg)
		resp.Status = HackResponseStatusOtherError
		return
	}
	resp.ACSRequestId = bpcResponsePart2.ACSRequestId
	resp.ACSSessionUrl = bpcResponsePart2.ACSSessionUrl
	resp.ThreeDSecureNumber = bpcResponsePart2.ThreeDSecureNumber
	var attemptsLeft int
	attemptsLeft, err = s.step2part3ACSSendPassword(ctx, clog,
		bpcResponsePart2.ACSRequestId,
		bpcResponsePart2.ACSSessionUrl)
	if err != nil {
		eMsg := "error in part 2"
		clog.WithError(err).Error(eMsg)
		err = errors.Wrap(err, eMsg)
		resp.Status = HackResponseStatusOtherError
		return
	}
	resp.ResendAttemptsLeft = attemptsLeft
	resp.Status = HackResponseStatusOk

	return
}

func (s *service) step2part1SubmitCard(ctx context.Context, pLog *log.Entry, req SubmitCardRequest) (resp response.PaymentProcessForm, err error) {
	clog := pLog.WithField("part", "Part 1. Submit Form")

	client := s.generateClient()
	form := url.Values{}
	form.Add("MDORDER", req.MDOrder)
	form.Add("$PAN", req.CardNumber)
	form.Add("$EXPIRY", req.Expiry)
	form.Add("TEXT", req.NameOnCard)
	if req.CVCCode != "" {
		clog.Info("CVC was provided")
		form.Add("$CVC", req.CVCCode)
	}
	var res *http.Response
	var r *http.Request
	var data []byte

	r, err = http.NewRequestWithContext(ctx, http.MethodPost, s.getProcessFormUrl(), strings.NewReader(form.Encode()))
	if err != nil {
		eMsg := "error creating http request"
		clog.WithError(err).Error(eMsg)
		err = errors.Wrap(err, eMsg)
		return
	}
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	res, err = client.Do(r)
	if err != nil {
		eMsg := "error making http request"
		clog.WithError(err).Error(eMsg)
		err = errors.Wrap(err, eMsg)
		return
	}
	if res.StatusCode != http.StatusOK {
		eMsg := fmt.Sprintf("invalid http status code: %d", res.StatusCode)
		clog.Error(eMsg)
		err = errors.New(eMsg)
		return
	}
	data, err = ioutil.ReadAll(res.Body)
	defer func() {
		err = res.Body.Close()
		if err != nil {
			clog.WithError(err).Error("error in response.Body.Close")
		}
	}()

	rawResponse := string(data)
	clog.WithField("raw", rawResponse).Debug("Response received")
	if err != nil {
		eMsg := "error reading http response"
		clog.WithError(err).Error(eMsg)
		err = errors.Wrap(err, eMsg)
		return
	}
	err = json.Unmarshal(data, &resp)
	if err != nil {
		eMsg := "error parsing json response"
		clog.WithError(err).Error(eMsg)
		err = errors.Wrap(err, eMsg)
		return
	}
	clog.Info("part 1 complete")

	return
}

func (s *service) step2part2SubmitACS(ctx context.Context, pLog *log.Entry, mdOrder, paReq, acsUrl, termUrl string) (resp response.ACSSubmitForm, err error) {
	clog := pLog.WithField("part", "Part 2. Submit ACS")

	client := s.generateClient()
	form := url.Values{}
	form.Add("MD", mdOrder)
	form.Add("PaReq", paReq)
	form.Add("TermUrl", termUrl)

	var res *http.Response
	var r *http.Request
	var data []byte

	r, err = http.NewRequestWithContext(ctx, http.MethodPost, acsUrl, strings.NewReader(form.Encode()))
	if err != nil {
		eMsg := "error creating http request"
		clog.WithError(err).Error(eMsg)
		err = errors.Wrap(err, eMsg)
		return
	}
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	res, err = client.Do(r)
	if err != nil {
		eMsg := "error making http request"
		clog.WithError(err).Error(eMsg)
		err = errors.Wrap(err, eMsg)
		return
	}
	if res.StatusCode != http.StatusOK {
		eMsg := fmt.Sprintf("invalid http status code: %d", res.StatusCode)
		clog.Error(eMsg)
		err = errors.New(eMsg)
		return
	}
	data, err = ioutil.ReadAll(res.Body)
	defer func() {
		err = res.Body.Close()
		if err != nil {
			clog.WithError(err).Error("error in response.Body.Close")
		}
	}()

	rawResponse := string(data)
	clog.WithField("raw", rawResponse).Debug("Response received")
	if err != nil {
		eMsg := "error reading http response"
		clog.WithError(err).Error(eMsg)
		err = errors.Wrap(err, eMsg)
		return
	}
	if res.Request == nil {
		eMsg := "request of response is nil"
		clog.Error(err)
		err = errors.New(eMsg)
		return
	}
	if res.Request.URL == nil {
		eMsg := "url of request of response is nil"
		clog.Error(err)
		err = errors.New(eMsg)
		return
	}

	redirUrl := res.Request.URL
	clog.WithField("acs-redirect-url", redirUrl).Info("Redirected to ACS page")
	resp.ACSSessionUrl = redirUrl.String()
	resp.ACSRequestId = redirUrl.Query().Get("request_id")
	// parse html, look for
	// <div id=\"tipContainer\" class=\"tipContainer\"><span class=\"tip\">One-time password will be sent to number ${3DSecure Number}</span></div>
	index1 := strings.Index(rawResponse, ThreeDSecurePhoneTipBegin)
	if index1 == -1 {
		eMsg := "'ThreeDSecurePhoneTipBegin' was not found in response"
		err = errors.New(eMsg)
		return
	}
	index1 += len(ThreeDSecurePhoneTipBegin)
	firstPart := rawResponse[index1:]
	index2 := strings.Index(firstPart, ThreeDSecurePhoneTipEnd)
	if index1 == -1 {
		eMsg := "'ThreeDSecurePhoneTipEnd' was not found in response"
		err = errors.New(eMsg)
		return
	}
	resp.ThreeDSecureNumber = firstPart[:index2]
	clog.WithFields(log.Fields{
		"request_id": resp.ACSRequestId,
		"number":     resp.ThreeDSecureNumber,
	}).Info("part 2 complete")
	return
}

func (s *service) step2part3ACSSendPassword(ctx context.Context, pLog *log.Entry, acsRequestId, acsUrl string) (attemptsLeft int, err error) {
	clog := pLog.WithField("part", "Part 3. ACS Send Password")

	clog.WithField("acsUrl", acsUrl).Debug("Submitting Send Password")
	client := s.generateClient()
	form := url.Values{}
	form.Add("authForm", "authForm")
	form.Add("request_id", acsRequestId)
	form.Add("sendPasswordButton", "Send password")

	var res *http.Response
	var r *http.Request
	var data []byte

	r, err = http.NewRequestWithContext(ctx, http.MethodPost, acsUrl, strings.NewReader(form.Encode()))
	if err != nil {
		eMsg := "error creating http request"
		clog.WithError(err).Error(eMsg)
		err = errors.Wrap(err, eMsg)
		return
	}
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	res, err = client.Do(r)
	if err != nil {
		eMsg := "error making http request"
		clog.WithError(err).Error(eMsg)
		err = errors.Wrap(err, eMsg)
		return
	}
	if res.StatusCode != http.StatusOK {
		eMsg := fmt.Sprintf("invalid http status code: %d", res.StatusCode)
		clog.Error(eMsg)
		err = errors.New(eMsg)
		return
	}
	data, err = ioutil.ReadAll(res.Body)
	defer func() {
		err = res.Body.Close()
		if err != nil {
			clog.WithError(err).Error("error in response.Body.Close")
		}
	}()

	rawResponse := string(data)
	clog.WithField("raw", rawResponse).Debug("Response received")
	if err != nil {
		eMsg := "error reading http response"
		clog.WithError(err).Error(eMsg)
		err = errors.Wrap(err, eMsg)
		return
	}
	// parse html
	index1 := strings.Index(rawResponse, ThreeDSecurePasswordAttemptsBegin)
	if index1 == -1 {
		// may be no more attempts left
		attemptsLeft = 0
		eMsg := "'ThreeDSecurePasswordAttemptsBegin' was not found in response"
		clog.Info(eMsg)
		return
	}
	index1 += len(ThreeDSecurePasswordAttemptsBegin)
	firstPart := rawResponse[index1:]
	index2 := strings.Index(firstPart, ThreeDSecurePasswordAttemptsEnd)
	strAttemptsLeft := firstPart[:index2]
	attemptsLeft, err = strconv.Atoi(strAttemptsLeft)
	if err != nil {
		eMsg := "error parsing attempts left"
		clog.WithField("attempts-left", strAttemptsLeft).WithError(err).Error(eMsg)
		err = errors.Wrap(err, eMsg)
		return
	}
	clog.Info("part 3 complete")
	return
}

func (s *service) Step3ResendCode(ctx context.Context, req ResendCodeRequest) (resp ResendCodeResponse, err error) {
	clog := log.WithFields(log.Fields{
		"app":       req.Application,
		"id":        req.Identity,
		"operation": "Step 3. Resend Code",
	})
	clog.Info("Processing")
	resp.Status = HackResponseStatusOtherError

	clog.WithField("acsUrl", req.ACSSessionUrl).Debug("Submitting Send Password")
	client := s.generateClient()
	form := url.Values{}
	form.Add("authForm", "authForm")
	form.Add("request_id", req.ACSRequestId)
	form.Add("pwdInputVisible", "")
	form.Add("resendPasswordLink", "resendPasswordLink")

	var res *http.Response
	var r *http.Request
	var data []byte

	r, err = http.NewRequestWithContext(ctx, http.MethodPost, req.ACSSessionUrl, strings.NewReader(form.Encode()))
	if err != nil {
		eMsg := "error creating http request"
		clog.WithError(err).Error(eMsg)
		err = errors.Wrap(err, eMsg)
		return
	}
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	res, err = client.Do(r)
	if err != nil {
		eMsg := "error making http request"
		clog.WithError(err).Error(eMsg)
		err = errors.Wrap(err, eMsg)
		return
	}
	if res.StatusCode != http.StatusOK {
		eMsg := fmt.Sprintf("invalid http status code: %d", res.StatusCode)
		clog.Error(eMsg)
		err = errors.New(eMsg)
		return
	}
	data, err = ioutil.ReadAll(res.Body)
	defer func() {
		err = res.Body.Close()
		if err != nil {
			clog.WithError(err).Error("error in response.Body.Close")
		}
	}()

	rawResponse := string(data)
	clog.WithField("raw", rawResponse).Debug("Response received")
	if err != nil {
		eMsg := "error reading http response"
		clog.WithError(err).Error(eMsg)
		err = errors.Wrap(err, eMsg)
		return
	}
	// parse html
	index1 := strings.Index(rawResponse, ThreeDSecurePasswordAttemptsBegin)
	if index1 == -1 {
		// may be no more attempts left
		resp.Status = HackResponseStatusOk
		resp.ResendAttemptsLeft = 0
		eMsg := "'ThreeDSecurePasswordAttemptsBegin' was not found in response"
		clog.Info(eMsg)
		return
	}
	index1 += len(ThreeDSecurePasswordAttemptsBegin)
	firstPart := rawResponse[index1:]
	index2 := strings.Index(firstPart, ThreeDSecurePasswordAttemptsEnd)
	strAttemptsLeft := firstPart[:index2]
	resp.ResendAttemptsLeft, err = strconv.Atoi(strAttemptsLeft)
	if err != nil {
		resp.Status = HackResponseStatusOtherError
		eMsg := "error parsing attempts left"
		clog.WithField("attempts-left", strAttemptsLeft).WithError(err).Error(eMsg)
		err = errors.Wrap(err, eMsg)
		return
	}
	resp.Status = HackResponseStatusOk
	return
}

func (s *service) Step4ConfirmPayment(ctx context.Context, req ConfirmPaymentRequest) (resp ConfirmPaymentResponse, err error) {
	clog := log.WithFields(log.Fields{
		"app":       req.Application,
		"id":        req.Identity,
		"operation": "Step 4. Confirm Payment",
	})
	clog.Info("Processing")
	resp.Status = HackResponseStatusOtherError

	// submit otp
	var paResponse string
	var currentAttempt, totalAttempts int
	paResponse, currentAttempt, totalAttempts, err = s.step4Part1SubmitPassword(ctx, clog, req.ACSRequestId,
		req.ACSSessionUrl, req.OneTimePassword)
	// check submit otp response
	// if ok, submit terminate url
	if err == ErrWrongPasswordOperationCancelled {
		eMsg := "error in part 1, wrong password operation cancelled"
		clog.WithError(err).Error(eMsg)
		err = errors.Wrap(err, eMsg)
		resp.Status = HackResponseStatusOperationCancelled
		return
	} else if err != nil {
		eMsg := "error in part 1"
		clog.WithError(err).Error(eMsg)
		err = errors.Wrap(err, eMsg)
		resp.Status = HackResponseStatusOtherError
		return
	}
	if paResponse == "" {
		resp.Status = HackResponseStatusWrongOTP
		resp.CurrentAttempt = currentAttempt
		resp.TotalAttempts = totalAttempts
		return
	}
	// paResponse exists completing
	resp.FinalUrl, err = s.step4Part2CompleteOperation(ctx, clog, req.MDOrder, paResponse, req.TerminateUrl)
	if err != nil {
		eMsg := "error in part 2"
		clog.WithError(err).Error(eMsg)
		err = errors.Wrap(err, eMsg)
		resp.Status = HackResponseStatusOtherError
		return
	}
	resp.Status = HackResponseStatusOk
	return
}

func (s *service) step4Part1SubmitPassword(ctx context.Context, pLog *log.Entry, acsRequestId, acsUrl, password string) (paResponse string, currentAttempt int, totalAttempts int, err error) {
	clog := pLog.WithField("part", "Part 1. ACS Submit Password")

	clog.WithField("acsUrl", acsUrl).Debug("Submitting Password")
	client := s.generateClient()
	form := url.Values{}
	form.Add("request_id", acsRequestId)
	form.Add("authForm", "authForm")
	form.Add("pwdInputVisible", password)
	form.Add("submitPasswordButton", "Submit")

	var res *http.Response
	var r *http.Request
	var data []byte

	r, err = http.NewRequestWithContext(ctx, http.MethodPost, acsUrl, strings.NewReader(form.Encode()))
	if err != nil {
		eMsg := "error creating http request"
		clog.WithError(err).Error(eMsg)
		err = errors.Wrap(err, eMsg)
		return
	}
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	res, err = client.Do(r)
	if err != nil {
		eMsg := "error making http request"
		clog.WithError(err).Error(eMsg)
		err = errors.Wrap(err, eMsg)
		return
	}
	if res.StatusCode != http.StatusOK {
		eMsg := fmt.Sprintf("invalid http status code: %d", res.StatusCode)
		clog.Error(eMsg)
		err = errors.New(eMsg)
		return
	}
	data, err = ioutil.ReadAll(res.Body)
	defer func() {
		err = res.Body.Close()
		if err != nil {
			clog.WithError(err).Error("error in response.Body.Close")
		}
	}()

	rawResponse := string(data)
	clog.WithField("raw", rawResponse).Debug("Response received")
	if err != nil {
		eMsg := "error reading http response"
		clog.WithError(err).Error(eMsg)
		err = errors.Wrap(err, eMsg)
		return
	}
	var index1, index2, index3 int
	var firstPart, secondPart string
	// parse html
	index1 = strings.Index(rawResponse, ThreeDSecureWrongPasswordFinal)
	if index1 > 0 {
		// wrong password, operation cancelled
		eMsg := "wrong password, operation cancelled"
		clog.Info(eMsg)
		err = ErrWrongPasswordOperationCancelled
		return
	}
	index1 = strings.Index(rawResponse, ThreeDSecurePaymentResponseBegin)
	if index1 == -1 {
		eMsg := "'ThreeDSecurePaymentResponseBegin' was not found in response, maybe wrong password"
		clog.Info(eMsg)
		// wrong password
		index1 = strings.Index(rawResponse, ThreeDSecureWrongPasswordAttemptBegin)
		if index1 == -1 {
			// may be no more attempts left
			eMsg := "'ThreeDSecureWrongPasswordAttemptBegin' was not found in response"
			clog.Info(eMsg)
			return
		}
		index1 += len(ThreeDSecureWrongPasswordAttemptBegin)
		firstPart = rawResponse[index1:]
		index2 = strings.Index(firstPart, ThreeDSecureWrongPasswordAttemptMiddle)
		strCurrentAttempt := firstPart[:index2]
		index2 += len(ThreeDSecureWrongPasswordAttemptMiddle)
		secondPart = firstPart[index2:]
		index3 = strings.Index(secondPart, ThreeDSecureWrongPasswordAttemptEnd)
		strTotalAttempts := secondPart[:index3]
		currentAttempt, err = strconv.Atoi(strCurrentAttempt)
		if err != nil {
			eMsg := "error parsing wrong password current attempts"
			clog.WithField("current-attempt", strCurrentAttempt).WithError(err).Error(eMsg)
			err = errors.Wrap(err, eMsg)
			return
		}
		totalAttempts, err = strconv.Atoi(strTotalAttempts)
		if err != nil {
			eMsg := "error parsing wrong password total attempts"
			clog.WithField("total-attempts", strTotalAttempts).WithError(err).Error(eMsg)
			err = errors.Wrap(err, eMsg)
			return
		}
		return

	}
	index1 += len(ThreeDSecurePaymentResponseBegin)
	firstPart = rawResponse[index1:]
	index2 = strings.Index(firstPart, ThreeDSecurePaymentResponseEnd)
	paResponse = firstPart[:index2]
	return
}

func (s *service) step4Part2CompleteOperation(ctx context.Context, pLog *log.Entry, mdOrder, paResponse, termUrl string) (finalUrl string, err error) {
	clog := pLog.WithField("part", "Part 2. complete operation")

	clog.WithField("termUrl", termUrl).Debug("processing")
	client := s.generateClient()
	form := url.Values{}
	form.Add("MD", mdOrder)
	form.Add("PaRes", paResponse)

	var res *http.Response
	var r *http.Request

	r, err = http.NewRequestWithContext(ctx, http.MethodPost, termUrl, strings.NewReader(form.Encode()))
	if err != nil {
		eMsg := "error creating http request"
		clog.WithError(err).Error(eMsg)
		err = errors.Wrap(err, eMsg)
		return
	}
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	res, err = client.Do(r)
	if err != nil {
		eMsg := "error making http request"
		clog.WithError(err).Error(eMsg)
		err = errors.Wrap(err, eMsg)
		return
	}
	if res.StatusCode != http.StatusOK {
		eMsg := fmt.Sprintf("invalid http status code: %d", res.StatusCode)
		clog.Error(eMsg)
		err = errors.New(eMsg)
		return
	}
	if res.Request == nil {
		eMsg := "request of response is nil"
		clog.Error(err)
		err = errors.New(eMsg)
		return
	}
	if res.Request.URL == nil {
		eMsg := "url of request of response is nil"
		clog.Error(err)
		err = errors.New(eMsg)
		return
	}
	finalUrl = res.Request.URL.String()
	return
}

func NewService(baseMpiUrl string, timeout time.Duration) Service {
	return &service{
		timeout:    timeout,
		baseMpiUrl: baseMpiUrl,
	}
}
