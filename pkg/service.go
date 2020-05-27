package pkg

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"

	"ykjam/bpchack/pkg/bpc/response"
)

type Service interface {
	Step1StartHack(ctx context.Context, req StartHackRequest) (StartHackResponse, error)
	Step2SubmitCard(ctx context.Context, req SubmitCardRequest) (SubmitCardResponse, error)
	Step3ResendCode(ctx context.Context) (err error)
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
const ThreeDSecureWrongNoPasswordAttemptText = `<div id="errorContainer" class="errorContainer"></div>`

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
	defer res.Body.Close()

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
	resp.RemainingTime = bpcResponse.RemainingSecs
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

	err = s.step2part3ACSSendPassword(ctx, clog,
		bpcResponsePart2.ACSRequestId,
		bpcResponsePart2.ACSSessionUrl)
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
	defer res.Body.Close()

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
	defer res.Body.Close()

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

func (s *service) step2part3ACSSendPassword(ctx context.Context, pLog *log.Entry, acsRequestId, acsUrl string) (err error) {
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
	defer res.Body.Close()

	rawResponse := string(data)
	clog.WithField("raw", rawResponse).Debug("Response received")
	if err != nil {
		eMsg := "error reading http response"
		clog.WithError(err).Error(eMsg)
		err = errors.Wrap(err, eMsg)
		return
	}
	clog.Info("part 3 complete")
	return
}

func (s *service) Step4ConfirmPayment(ctx context.Context, req ConfirmPaymentRequest) (resp ConfirmPaymentResponse, err error) {
	clog := log.WithFields(log.Fields{
		"app":       req.Application,
		"id":        req.Identity,
		"operation": "Step 3. Confirm Payment",
	})
	clog.Info("Processing")

	// submit otp
	// check submit otp response
	// if ok, submit terminate url
	return
}

func NewService(baseMpiUrl string, timeout time.Duration) Service {
	return &service{
		timeout:    timeout,
		baseMpiUrl: baseMpiUrl,
	}
}
