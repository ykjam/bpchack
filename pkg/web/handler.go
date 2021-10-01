package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"time"

	log "github.com/sirupsen/logrus"

	"ykjam/bpchack/pkg"
)

type HandlerContext interface {
	HandleUtilityEpoch(w http.ResponseWriter, r *http.Request)
	HandleUtilityIP(w http.ResponseWriter, r *http.Request)
	HandleStartHack(w http.ResponseWriter, r *http.Request)
	HandleSubmitCard(w http.ResponseWriter, r *http.Request)
	HandleResendCode(w http.ResponseWriter, r *http.Request)
	HandleConfirmPayment(w http.ResponseWriter, r *http.Request)
}

type handlerContext struct {
	service      pkg.Service
	rApplication *regexp.Regexp
	rIdentity    *regexp.Regexp
	rCardNumber  *regexp.Regexp
	rCardExpiry  *regexp.Regexp
	rCardCVC     *regexp.Regexp
}

type httpPostWithLog func(w http.ResponseWriter, r *http.Request, ctx context.Context, clog *log.Entry)

func GetRemoteAddress(r *http.Request) string {
	if val := r.Header.Get("X-Forwarded-For"); val != "" {
		return val
	} else if val := r.Header.Get("X-Real-IP"); val != "" {
		return val
	} else {
		return r.RemoteAddr
	}
}

func errorHandler(w http.ResponseWriter, status int) {
	w.WriteHeader(status)
	if status == http.StatusNotFound {
		_, _ = fmt.Fprint(w, "Page not found")

	} else {
		_, _ = fmt.Fprintf(w, "HTTP %d error", status)
	}
}

func errorHandlerWithError(w http.ResponseWriter, status int, err error) {
	w.WriteHeader(status)
	_, _ = fmt.Fprintf(w, "HTTP %d error\nError %v", status, err)
}

func responseWithCodeAndMessage(w http.ResponseWriter, status int, message string) {
	w.WriteHeader(status)
	_, _ = fmt.Fprintln(w, message)
}

func jsonResponse(clog *log.Entry, w http.ResponseWriter, response interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	err := json.NewEncoder(w).Encode(response)
	if err != nil {
		clog.WithError(err).Error("error in json.Encode")
	}
}

func (c *handlerContext) handleHttpPostWithLog(handleName string, w http.ResponseWriter, r *http.Request, f httpPostWithLog) {
	ctx := r.Context()
	clog := log.WithFields(log.Fields{
		"remote-addr": GetRemoteAddress(r),
		"uri":         r.RequestURI,
		"method":      r.Method,
		"handle":      handleName,
	}).WithContext(ctx)
	if r.Method == http.MethodPost {
		f(w, r, ctx, clog)
	} else {
		clog.Error("invalid request, method not allowed")
		errorHandler(w, http.StatusMethodNotAllowed)
	}
}

func (c *handlerContext) isApplicationAndIdentityValid(application, identity string) bool {
	if !c.rApplication.MatchString(application) {
		return false
	} else if !c.rIdentity.MatchString(identity) {
		return false
	}
	return true
}

func (c *handlerContext) isCardValid(clog *log.Entry, cardNumber, cardExpiry, nameOnCard, cvcCode string) bool {
	if !c.rCardNumber.MatchString(cardNumber) {
		clog.WithField("card-number", cardNumber).Error("card number validation failed")
		return false
	} else if !c.rCardExpiry.MatchString(cardExpiry) {
		clog.WithField("card-expiry", cardExpiry).Error("card expiry validation failed")
		return false
	} else if len(nameOnCard) < 4 || len(nameOnCard) > 32 {
		clog.WithField("name-on-card", nameOnCard).Error("name on card validation failed")
		return false
	} else if cvcCode != "" && !c.rCardCVC.MatchString(cvcCode) {
		clog.WithField("cvc", cvcCode).Error("cvc code validation failed")
		return false
	}
	return true
}

func (c *handlerContext) HandleStartHack(w http.ResponseWriter, r *http.Request) {
	h := "handleStartHack"
	c.handleHttpPostWithLog(h, w, r, func(w http.ResponseWriter, r *http.Request, ctx context.Context, clog *log.Entry) {
		// request parameters
		application := r.FormValue("app")
		identity := r.FormValue("id")
		paymentUrl := r.FormValue("url")
		// validate inputs
		if !c.isApplicationAndIdentityValid(application, identity) {
			clog.Warn("not valid application or identity, ignoring request")
			errorHandler(w, http.StatusBadRequest)
			return
		}
		clog.WithFields(log.Fields{
			"application": application,
			"identity":    identity,
		}).Debug("request received")
		resp, err := c.service.Step1StartHack(ctx, pkg.StartHackRequest{
			Application: application,
			Identity:    identity,
			PaymentUrl:  paymentUrl,
		})
		if err != nil {
			clog.WithError(err).Error("step1 start hack failed")
			errorHandlerWithError(w, http.StatusInternalServerError, err)
			return
		}
		jsonResponse(clog, w, resp)
	})
}

func (c *handlerContext) HandleSubmitCard(w http.ResponseWriter, r *http.Request) {
	h := "handleSubmitCard"
	c.handleHttpPostWithLog(h, w, r, func(w http.ResponseWriter, r *http.Request, ctx context.Context, clog *log.Entry) {
		// request parameters
		application := r.FormValue("app")
		identity := r.FormValue("id")
		mdOrder := r.FormValue("md-order")
		cardNumber := r.FormValue("card-number")
		cardExpiry := r.FormValue("card-expiry")
		nameOnCard := r.FormValue("name-on-card")
		cvcCode := r.FormValue("card-cvc")
		// validate inputs
		if !c.isApplicationAndIdentityValid(application, identity) {
			clog.Warn("not valid application or identity, ignoring request")
			errorHandler(w, http.StatusBadRequest)
			return
		}
		if !c.isCardValid(clog, cardNumber, cardExpiry, nameOnCard, cvcCode) {
			clog.Warn("not valid card details, ignoring request")
			errorHandler(w, http.StatusBadRequest)
			return
		}
		clog.WithFields(log.Fields{
			"application": application,
			"identity":    identity,
		}).Debug("request received")
		resp, err := c.service.Step2SubmitCard(ctx, pkg.SubmitCardRequest{
			Application: application,
			Identity:    identity,
			MDOrder:     mdOrder,
			CardNumber:  cardNumber,
			Expiry:      cardExpiry,
			NameOnCard:  nameOnCard,
			CVCCode:     cvcCode,
		})
		if err != nil {
			clog.WithError(err).Error("step2 submit card failed")
			errorHandlerWithError(w, http.StatusInternalServerError, err)
			return
		}
		jsonResponse(clog, w, resp)
	})
}

func (c *handlerContext) HandleResendCode(w http.ResponseWriter, r *http.Request) {
	h := "handleResendCode"
	c.handleHttpPostWithLog(h, w, r, func(w http.ResponseWriter, r *http.Request, ctx context.Context, clog *log.Entry) {
		// request parameters
		application := r.FormValue("app")
		identity := r.FormValue("id")
		acsRequestId := r.FormValue("acs-req-id")
		acsSessionUrl := r.FormValue("acs-session-url")
		// validate inputs
		if !c.isApplicationAndIdentityValid(application, identity) {
			clog.Warn("not valid application or identity, ignoring request")
			errorHandler(w, http.StatusBadRequest)
			return
		}
		clog.WithFields(log.Fields{
			"application": application,
			"identity":    identity,
		}).Debug("request received")
		resp, err := c.service.Step3ResendCode(ctx, pkg.ResendCodeRequest{
			Application:   application,
			Identity:      identity,
			ACSRequestId:  acsRequestId,
			ACSSessionUrl: acsSessionUrl,
		})
		if err != nil {
			clog.WithError(err).Error("step3 resend code failed")
			errorHandlerWithError(w, http.StatusInternalServerError, err)
			return
		}
		jsonResponse(clog, w, resp)
	})
}

func (c *handlerContext) HandleConfirmPayment(w http.ResponseWriter, r *http.Request) {
	h := "handleConfirmPayment"
	c.handleHttpPostWithLog(h, w, r, func(w http.ResponseWriter, r *http.Request, ctx context.Context, clog *log.Entry) {
		// request parameters
		application := r.FormValue("app")
		identity := r.FormValue("id")
		mdOrder := r.FormValue("md-order")
		acsRequestId := r.FormValue("acs-req-id")
		acsSessionUrl := r.FormValue("acs-session-url")
		oneTimePassword := r.FormValue("otp")
		terminateUrl := r.FormValue("term-url")
		// validate inputs
		if !c.isApplicationAndIdentityValid(application, identity) {
			clog.Warn("not valid application or identity, ignoring request")
			errorHandler(w, http.StatusBadRequest)
			return
		}
		clog.WithFields(log.Fields{
			"application": application,
			"identity":    identity,
		}).Debug("request received")
		resp, err := c.service.Step4ConfirmPayment(ctx, pkg.ConfirmPaymentRequest{
			Application:     application,
			Identity:        identity,
			MDOrder:         mdOrder,
			ACSRequestId:    acsRequestId,
			ACSSessionUrl:   acsSessionUrl,
			OneTimePassword: oneTimePassword,
			TerminateUrl:    terminateUrl,
		})
		if err != nil {
			clog.WithError(err).Error("step4 confirm payment failed")
			errorHandlerWithError(w, http.StatusInternalServerError, err)
			return
		}
		jsonResponse(clog, w, resp)
	})
}

func (c *handlerContext) HandleUtilityEpoch(w http.ResponseWriter, _ *http.Request) {
	epoch := time.Now().Unix()
	responseWithCodeAndMessage(w, http.StatusOK, fmt.Sprintf("%d", epoch))
}

func (c *handlerContext) HandleUtilityIP(w http.ResponseWriter, r *http.Request) {
	remoteIp := GetRemoteAddress(r)
	responseWithCodeAndMessage(w, http.StatusOK, remoteIp)
}

func NewHandlerContext(service pkg.Service) HandlerContext {
	return &handlerContext{
		service:      service,
		rApplication: regexp.MustCompile(`(?i)[a-z0-9]{3,16}`),
		rIdentity:    regexp.MustCompile(`(?i)[a-z0-9]{3,64}`),
		rCardNumber:  regexp.MustCompile(`[0-9]{16}`),
		rCardExpiry:  regexp.MustCompile(`[0-9]{6}`),
		rCardCVC:     regexp.MustCompile(`[0-9]{3}`),
	}
}
