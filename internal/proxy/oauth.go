package proxy

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/ananchev/homeconnect-proxy/internal/logger"
)

const (
	TokenURL = "https://api.home-connect.com/security/oauth/token"
	BaseURL  = "https://api.home-connect.com/api"
)

type ClientData struct {
	ClientId     string
	ClientSecret string
	ClientScopes string
}

var clientData ClientData
var routes string

// Serialize token to disk for access when application is restarted, and for using the refresh token
func cacheToken(token Token) (err error) {

	data, err := json.Marshal(token)
	if err != nil {
		logger.Error("Error encoding token json to save to disk: {err}", "err", err.Error())
		return
	}

	err = ioutil.WriteFile("data/token.cache", data, 0644)
	if err != nil {
		logger.Error("Error saving token to disk: {err}", "err", err.Error())
		return
	}
	return
}

// Load the cached token file to use its refresh token
func loadCachedToken() (token Token, err error) {
	file, err := ioutil.ReadFile("data/token.cache")
	if err != nil {
		logger.Error("Error reading token cache file from disk: {err}", "err", err.Error())
		return
	}
	err = json.Unmarshal([]byte(file), &token)
	if err != nil {
		logger.Error("Error unmarshalling token cache file: {err}", "err", err.Error())
		return
	}
	return
}

func getToken() (token Token, err error) {
	token, err = loadCachedToken()
	if err != nil {
		err_descr := "Error getting token: " + err.Error()
		logger.Error(err_descr)
		return
	}

	if epochSeconds() > token.ExpiresAt { //token has expired, refresh it
		logger.Info("Access token has expired, initiating refresh...")
		if token.RefreshToken == "" {
			err_descr := "Refresh Token Not Found. Please re-authorize the application."
			logger.Error(err_descr)
			err = errors.New(err_descr)
			return
		}
		var newToken Token
		err = requestToken("REFRESH", token.RefreshToken, &newToken)
		if err != nil {
			logger.Info("Error getting new access token from refresh token: {error}", "error", err)
			return
		}
		err = cacheToken(newToken)
		if err != nil {
			return
		}
		token = newToken
	}
	return
}

// Get initial auth token, or refresh it using refresh token from cache
func requestToken(requestType string, code string, token *Token) (err error) {

	logger.Info("Requesting new '{token}' token for API access ...", "token", requestType)
	// initate the payload values map, will add all values in the switch below, depending if requesting a new token, or refreshing it
	values := url.Values{}
	values.Set("client_secret", clientData.ClientSecret)

	switch requestType {
	case "AUTHORIZE":
		values.Set("client_id", clientData.ClientId)
		values.Set("grant_type", "authorization_code")
		values.Set("code", code)
	case "REFRESH":
		values.Set("grant_type", "refresh_token")
		values.Set("refresh_token", code)
	default:
		errDescr := "unknown token type"
		logger.Error("Get token error: {err}", "err", errDescr)
		err = errors.New(errDescr)
		return
	}

	// form a request with URL-encoded payload
	req, err := http.NewRequest(http.MethodPost, TokenURL, strings.NewReader(values.Encode()))
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	// send out the HTTP request
	logger.Info("Sending the HTTP request ...", "request_type", requestType)
	httpClient := http.Client{}
	resp, err := httpClient.Do(req)

	if err != nil {
		logger.Error("Error with token request: {error}", "error", err.Error())
		return
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)

	if resp.StatusCode != 200 {
		err_descr := string(body)
		logger.Error("{resp_status}: {error}", "resp_status", resp.Status, "error", err_descr)
		err = errors.New(err_descr)
		return
	} else {
		logger.Info(resp.Status)
	}

	err = json.Unmarshal(body, &token)

	// decoder error
	if err != nil {
		err_descr := "Error decoding token request response: " + err.Error()
		logger.Error(err_descr)
		err = errors.New(err_descr)
		return
	}

	//if no access token
	if token.AccessToken == "" {
		err_descr := "Missing access token: " + string(body)
		logger.Error(err_descr)
		err = errors.New(err_descr)
		return
	}

	// calculate expiry
	const DELTASECS = 5       // remove 5 seconds from the exipry to account for transport delay
	if token.ExpiresIn == 0 { //no expiration, so make it a year
		token.ExpiresIn = 31536000
	}
	logger.Info("token expires in '{exp}", "exp", token.ExpiresIn)

	token.ExpiresAt = epochSeconds() + int64(token.ExpiresIn) - DELTASECS

	t := *token //copy into t the value of the struct pointed to by token
	err = cacheToken(t)
	if err != nil {
		err_descr := "Error saving token data: " + err.Error()
		logger.Error(err_descr)
		err = errors.New(err_descr)
		return
	}

	logger.Info("Completed getToken request for '{request_type}'", "request_type", requestType)
	return
}

//get Access Token, refresh if expired, set header bearer token
func setHeader(newReq *http.Request) (err error) {
	token, err := getToken()
	if err != nil {
		err_descr := "Error getting access token: " + err.Error()
		logger.Error(err_descr)
		err = errors.New(err_descr)
		return
	}
	newReq.Header.Add("Authorization", "Bearer "+token.AccessToken)
	newReq.Header.Set("Content-Type", "application/json")
	// newReq.Header.Set("Accept", "application/json")
	return
}

func epochSeconds() int64 {
	now := time.Now()
	secs := now.Unix()
	return secs
}

type Token struct {
	AccessToken  string `json:"access_token"`
	ExpiresAt    int64  `json:"expires_at"`
	ExpiresIn    int    `json:"expires_in"`
	IdToken      string `json:"id_token"`
	RefreshToken string `json:"refresh_token"`
	Scope        string `json:"scope"`
	TokenType    string `json:"token_type"`
}
