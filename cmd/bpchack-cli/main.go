package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"

	"ykjam/bpchack/pkg"
)

func run() error {
	log.Info("Starting BPC Hack CLI")
	complete := false

	var input string
	var err error
	var reader *bufio.Reader
	var service pkg.Service
	var step1Response pkg.StartHackResponse
	var step2Response pkg.SubmitCardResponse
	err = godotenv.Load()
	if err != nil {
		log.WithError(err).Error("error loading .env, ignoring")
	}
	reader = bufio.NewReader(os.Stdin)
	ctx := context.Background()
	var mpiBaseUrl, paymentUrl, cardNumber, cardExpiry, nameOnCard, cardCvc string
	mpiBaseUrl = os.Getenv("MPI_BASE_URL")
	cardNumber = os.Getenv("CARD_NUMBER")
	nameOnCard = os.Getenv("NAME_ON_CARD")
	cardExpiry = os.Getenv("CARD_EXPIRY")

mainLoop:
	for !complete {
		for !complete {
			if mpiBaseUrl != "" {
				fmt.Printf("mpi base url [%s] > ", mpiBaseUrl)
			} else {
				fmt.Print("mpi base url > ")
			}

			input, err = reader.ReadString('\n')
			input = strings.TrimSpace(input)
			if err != nil {
				eMsg := "error reading mpi base url, leaving"
				log.WithError(err).Error(eMsg)
				return errors.Wrap(err, eMsg)
			}
			if input != "" {
				mpiBaseUrl = input
			}
			// verify url here
			if !strings.HasPrefix(mpiBaseUrl, "https://") || len(mpiBaseUrl) < 12 {
				eMsg := "please verify mpi base url"
				fmt.Println(eMsg)
				continue
			}
			complete = true
		}
		service = pkg.NewService(mpiBaseUrl, 30*time.Second)
		log.Info("service initialized")
		complete = false
		for !complete {
			if paymentUrl != "" {
				fmt.Printf("payment url [%s] > ", paymentUrl)
			} else {
				fmt.Print("payment url > ")
			}
			input, err = reader.ReadString('\n')
			if err != nil {
				eMsg := "error reading payment url, leaving"
				log.WithError(err).Error(eMsg)
				return errors.Wrap(err, eMsg)
			}
			input = strings.TrimSpace(input)
			if input != "" {
				paymentUrl = input
			}
			complete = true
		}
		step1Request := pkg.StartHackRequest{
			Application: "bpchack-cli",
			Identity:    os.Getenv("USER"),
			PaymentUrl:  paymentUrl,
		}

		step1Response, err = service.Step1StartHack(ctx, step1Request)
		if err != nil {
			eMsg := "error executing step1 start hack, restarting"
			log.WithError(err).Error(eMsg)
			complete = false
			continue mainLoop
		}
		fmt.Printf("response: %v\n\n", step1Response)

		if cardNumber != "" {
			fmt.Printf("Card Number [%s] > ", cardNumber)
		} else {
			fmt.Print("Card Number > ")
		}
		input, err = reader.ReadString('\n')
		if err != nil {
			eMsg := "error reading card number, leaving"
			log.WithError(err).Error(eMsg)
			return errors.Wrap(err, eMsg)
		}
		input = strings.TrimSpace(input)
		if input != "" {
			cardNumber = input
		}

		if nameOnCard != "" {
			fmt.Printf("Name on Card [%s] > ", nameOnCard)
		} else {
			fmt.Print("Name on Card > ")
		}
		input, err = reader.ReadString('\n')
		if err != nil {
			eMsg := "error reading name on card, leaving"
			log.WithError(err).Error(eMsg)
			return errors.Wrap(err, eMsg)
		}
		input = strings.TrimSpace(input)
		if input != "" {
			nameOnCard = input
		}

		if cardExpiry != "" {
			fmt.Printf("Card Expiry [%s] > ", cardExpiry)
		} else {
			fmt.Print("Card Expiry > ")
		}
		input, err = reader.ReadString('\n')
		if err != nil {
			eMsg := "error reading card expiry, leaving"
			log.WithError(err).Error(eMsg)
			return errors.Wrap(err, eMsg)
		}
		input = strings.TrimSpace(input)
		if input != "" {
			cardExpiry = input
		}

		fmt.Print("Card CVC > ")
		input, err = reader.ReadString('\n')
		if err != nil {
			eMsg := "error reading card expiry, leaving"
			log.WithError(err).Error(eMsg)
			return errors.Wrap(err, eMsg)
		}
		input = strings.TrimSpace(input)
		cardCvc = input

		step2Request := pkg.SubmitCardRequest{
			Application: step1Request.Application,
			Identity:    step1Request.Identity,
			MDOrder:     step1Response.MDOrder,
			CardNumber:  cardNumber,
			Expiry:      cardExpiry,
			NameOnCard:  nameOnCard,
			CVCCode:     cardCvc,
		}

		step2Response, err = service.Step2SubmitCard(ctx, step2Request)
		if err != nil {
			eMsg := "error executing step2 submit card, restarting"
			log.WithError(err).Error(eMsg)
			complete = false
			continue mainLoop
		}
		fmt.Printf("response: %v", step2Response)

	}
	return nil
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "%v", err)
		os.Exit(1)
	}
}
