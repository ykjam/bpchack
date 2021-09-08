package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"

	"ykjam/bpchack/pkg"
	"ykjam/bpchack/pkg/web"
)

type config struct {
	ListenAddress string `json:"listen_address"`
	BaseMpiUrl    string `json:"base_mpi_url,omitempty"`
}

func ReadConfig(source string) (c *config, err error) {
	var raw []byte
	raw, err = ioutil.ReadFile(source)
	if err != nil {
		eMsg := "error reading config from file"
		log.WithError(err).Error(eMsg)
		err = errors.Wrap(err, eMsg)
		return
	}
	err = json.Unmarshal(raw, &c)
	if err != nil {
		eMsg := "error parsing config from json"
		log.WithError(err).Error(eMsg)
		err = errors.Wrap(err, eMsg)
		c = nil
	}
	return
}

func run() error {
	log.Info("Starting BPC Hack proxy")
	signalChan := make(chan os.Signal, 1)
	quitChan := make(chan interface{})

	var configFile string
	var conf *config
	var err error
	err = godotenv.Load()
	if err != nil {
		log.WithError(err).Error("error loading .env, ignoring")
	}
	configFile = os.Getenv("BPCHACK_CONFIG_FILE")
	if configFile == "" {
		configFile = "config.json"
	}

	conf, err = ReadConfig(configFile)
	if err != nil {
		log.WithError(err).WithField("config-file", configFile).Error("error loading configuration")
		return err
	}
	service := pkg.NewService(conf.BaseMpiUrl, 60*time.Second)
	log.Info("service initialized")

	hc := web.NewHandlerContext(service)

	sm := http.NewServeMux()
	sm.HandleFunc("/api/epoch", hc.HandleUtilityEpoch)
	sm.HandleFunc("/api/ip", hc.HandleUtilityIP)
	sm.HandleFunc("/api/v1/start-hack", hc.HandleStartHack)
	sm.HandleFunc("/api/v1/submit-card", hc.HandleSubmitCard)
	sm.HandleFunc("/api/v1/resend-code", hc.HandleResendCode)
	sm.HandleFunc("/api/v1/confirm-payment", hc.HandleConfirmPayment)

	server := http.Server{
		Addr:              conf.ListenAddress,
		Handler:           sm,
		ReadTimeout:       60 * time.Second,
		ReadHeaderTimeout: 30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
	var listener net.Listener
	listener, err = net.Listen("tcp", conf.ListenAddress)
	if err != nil {
		log.WithError(err).Error("error setting up listener")
		return err
	}
	log.WithField("listen", conf.ListenAddress).Info("Starting HTTP API server")
	go startServer(&server, listener)
	for {
		select {
		case <-quitChan:
			log.Warn("quit channel closed, closing listener")
			err = server.Shutdown(context.Background())
			if err != nil {
				log.WithError(err).Error("error during HTTP server shutdown")
				return err
			}
			return nil
		case sig := <-signalChan:
			switch sig {
			case os.Interrupt, os.Kill, syscall.SIGTERM:
				log.Info("interrupt signal received, sending Quit signal")
				close(quitChan)
			}
		}
	}
}

func startServer(srv *http.Server, listener net.Listener) {
	err := srv.Serve(listener)
	if err != nil {
		log.WithError(err).Error("HTTP API server error")
	}
	log.Warn("closing HTTP API server")
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}
