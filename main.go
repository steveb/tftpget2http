package main

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/pin/tftp"
)

type Config struct {
	ListenAddr  string
	HTTPTimeout int
	HTTPMaxIdle int
	TFTPTimeout int
	TFTPRetries int
	HTTPUrl     string
}

type EnvVar struct {
	DefaultValue string
	Help         string
	Mandatory    bool
}

var configDefaults map[string]EnvVar

var config *Config

var client *http.Client

func printUsage() {
	fmt.Print("Usage:\n\n")
	var defaultText string

	for k, v := range configDefaults {
		if v.Mandatory {
			defaultText = "Mandatory"
		} else {
			defaultText = fmt.Sprintf("Default %s", v.DefaultValue)
		}
		fmt.Printf("%s: %s (%s)\n\n", k, v.Help, defaultText)
	}

}

func getEnv(key string) (string, error) {
	value := os.Getenv(key)
	configDefault := configDefaults[key]
	if value == "" {
		if configDefault.Mandatory {
			return "", fmt.Errorf("%s: mandatory value missing", key)
		}
		value = configDefault.DefaultValue
	}
	fmt.Printf("%s=%s\n", key, value)
	return value, nil
}

func getIntEnv(key string) (int, error) {
	strValue, err := getEnv(key)
	if err != nil {
		return 0, err
	}
	value, err := strconv.Atoi(strValue)
	if err != nil {
		return 0, fmt.Errorf("%s: %s", key, err)
	}
	return value, nil
}

func buildConfig() error {
	configDefaults = make(map[string]EnvVar)
	configDefaults["HTTP_URL"] = EnvVar{
		Help:      "Address and port to listen to",
		Mandatory: true,
	}
	configDefaults["LISTEN"] = EnvVar{
		DefaultValue: ":69",
		Help:         "Address and port to listen to",
	}
	configDefaults["HTTP_MAX_IDLE"] = EnvVar{
		DefaultValue: "10",
		Help:         "Maximum idle connections for HTTP requests",
	}
	configDefaults["HTTP_TIMEOUT"] = EnvVar{
		DefaultValue: "5",
		Help:         "HTTP timeout limit of initial response in seconds",
	}
	configDefaults["TFTP_TIMEOUT"] = EnvVar{
		DefaultValue: "1",
		Help:         "TFTP timeout limit in seconds",
	}
	configDefaults["TFTP_RETRIES"] = EnvVar{
		DefaultValue: "5",
		Help:         "TFTP retries",
	}

	var err error
	config = &Config{}
	config.ListenAddr, err = getEnv("LISTEN")
	if err != nil {
		return err
	}
	config.HTTPMaxIdle, err = getIntEnv("HTTP_MAX_IDLE")
	if err != nil {
		return err
	}
	config.HTTPTimeout, err = getIntEnv("HTTP_TIMEOUT")
	if err != nil {
		return err
	}
	config.TFTPTimeout, err = getIntEnv("TFTP_TIMEOUT")
	if err != nil {
		return err
	}
	config.TFTPRetries, err = getIntEnv("TFTP_RETRIES")
	if err != nil {
		return err
	}
	config.HTTPUrl, err = getEnv("HTTP_URL")
	if err != nil {
		return err
	}
	_, err = stringToUrl(config.HTTPUrl)
	if err != nil {
		return fmt.Errorf("HTTP_URL: %s", err)
	}
	return nil
}

func stringToUrl(stringUrl string) (*url.URL, error) {
	parsedUrl, err := url.Parse(stringUrl)
	if err != nil {
		return parsedUrl, fmt.Errorf("error parsing: '%s'", stringUrl)
	}
	if parsedUrl.Scheme != "http" && parsedUrl.Scheme != "https" {
		return parsedUrl, fmt.Errorf("invalid scheme: '%s'", parsedUrl.Scheme)
	}
	if len(parsedUrl.Host) == 0 {
		return parsedUrl, fmt.Errorf("host must be provided")
	}
	return parsedUrl, nil
}

func setForwardedHeader(req *http.Request, from string) {
	req.Header.Add("X-Forwarded-From", from)
	req.Header.Add("X-Forwarded-Proto", "tftp")
	req.Header.Add("Forwarded", "for=\""+from+"\",proto=tftp")
}

func readHandler(filename string, rf io.ReaderFrom) error {
	raddr := rf.(tftp.OutgoingTransfer).RemoteAddr()
	from := raddr.String()

	log.Printf("%s - received RRQ '%s'", from, filename)

	// TODO(sbaker): replace with url JoinPath when go1.19 is consumable
	u, err := stringToUrl(config.HTTPUrl + filename)
	if err != nil {
		log.Printf("%s - error parsing URL: %s", from, err)
		return err
	}
	log.Printf("%s - %s GET", from, u)

	start := time.Now()
	var n int64
	defer func() {
		if n == 0 {
			return
		}
		elapsed := time.Since(start)
		log.Printf("%s - completed RRQ '%s' bytes=%d duration=%s", from, filename, n, elapsed)
	}()

	req, err := http.NewRequest("GET", u.String(), nil)
	if err != nil {
		log.Printf("error creating request: %s", err)
		return err
	}
	setForwardedHeader(req, from)

	res, err := client.Do(req)
	if err != nil {
		log.Printf("%s - error on HTTP GET: %s", from, err)
		return err
	}
	defer res.Body.Close()

	if res.StatusCode == 404 {
		log.Printf("%s - '%s' not found", from, u)
		return fmt.Errorf("file not found")
	} else if res.StatusCode != 200 {
		log.Printf("%s - unexpected response status code is: %d", from, res.StatusCode)
		return fmt.Errorf("unexpected response code: %d", res.StatusCode)
	}

	if res.ContentLength >= 0 {
		rf.(tftp.OutgoingTransfer).SetSize(res.ContentLength)
	}

	n, err = rf.ReadFrom(res.Body)
	if err != nil {
		log.Printf("%s -  ReadFrom returned error: %s", from, err)
		return err
	}

	return nil
}

func writeHandler(filename string, wt io.WriterTo) error {
	raddr := wt.(tftp.IncomingTransfer).RemoteAddr()
	from := raddr.String()
	log.Printf("%s - received unsupported WRQ '%s'", from, filename)
	return errors.New("put is unsupported")
}

func main() {
	err := buildConfig()
	if err != nil {
		fmt.Printf("Error parsing environment variables:\n%s\n\n", err)
		printUsage()
		os.Exit(1)
	}

	client = &http.Client{
		Transport: &http.Transport{
			MaxIdleConnsPerHost:   config.HTTPMaxIdle,
			ResponseHeaderTimeout: time.Duration(config.HTTPTimeout) * time.Second,
		},
	}

	s := tftp.NewServer(readHandler, writeHandler)
	s.SetTimeout(time.Duration(config.TFTPTimeout) * time.Second)
	s.SetRetries(config.TFTPRetries)

	log.Printf("proxying TFTP requests on %s to %s", config.ListenAddr, config.HTTPUrl)
	err = s.ListenAndServe(config.ListenAddr)
	if err != nil {
		log.Fatalf("Starting TFTP server failed: %v\n", err)
	}
}
