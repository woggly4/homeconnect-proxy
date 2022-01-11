package proxy

import (
	"bytes"
	"encoding/json"
	"errors"
	"text/template"

	"io/ioutil"
	"net/http"
	"net/url"

	"strings"
	"time"

	"github.com/ananchev/homeconnect-proxy/internal/logger"
	"github.com/gorilla/mux"
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

// Documentation: https://api-docs.home-connect.com/authorization
// Authorization URL: https://api.home-connect.com/security/oauth/authorize
// Token URL: https://api.home-connect.com/security/oauth/token

func Run(port string, hcClientId, hcClientSecret, hcClientScopes string) {
	clientData.ClientId = hcClientId
	clientData.ClientSecret = hcClientSecret
	clientData.ClientScopes = hcClientScopes

	r := mux.NewRouter()
	// proxy-specific routes
	r.HandleFunc("/", homePageHandler)
	r.HandleFunc("/proxy/auth", authPageHandler)
	r.HandleFunc("/proxy/auth/redirect", redirectHandler)
	r.HandleFunc("/proxy/success", authSuccessPageHandler)

	// routes below will be redirected to home connect
	// documentation available at https://apiclient.home-connect.com/

	// default
	r.HandleFunc("/homeappliances", redirectToHomeConnect).Methods("GET")
	r.HandleFunc("/homeappliances/{.*}", redirectToHomeConnect).Methods("GET")

	// programs
	r.HandleFunc("/homeappliances/{.*}/programs", redirectToHomeConnect).Methods("GET")
	r.HandleFunc("/homeappliances/{.*}/programs/available", redirectToHomeConnect).Methods("GET")
	r.HandleFunc("/homeappliances/{.*}/programs/available/{.*}", redirectToHomeConnect).Methods("GET")
	r.HandleFunc("/homeappliances/{.*}/programs/active", redirectToHomeConnect).Methods("GET", "PUT", "DELETE")
	r.HandleFunc("/homeappliances/{.*}/programs/active/options", redirectToHomeConnect).Methods("GET", "PUT")
	r.HandleFunc("/homeappliances/{.*}/programs/active/options/{.*}", redirectToHomeConnect).Methods("GET", "PUT")
	r.HandleFunc("/homeappliances/{.*}/programs/selected", redirectToHomeConnect).Methods("GET", "PUT")
	r.HandleFunc("/homeappliances/{.*}/programs/selected/options", redirectToHomeConnect).Methods("GET", "PUT")
	r.HandleFunc("/homeappliances/{.*}/programs/selected/options/{.*}", redirectToHomeConnect).Methods("GET", "PUT")

	// status_events
	r.HandleFunc("/homeappliances/{.*}/status", redirectToHomeConnect).Methods("GET")
	r.HandleFunc("/homeappliances/{.*}/status/{.*}", redirectToHomeConnect).Methods("GET")
	r.HandleFunc("/homeappliances/events", redirectToHomeConnect).Methods("GET")

	// images
	r.HandleFunc("/homeappliances/{.*}/images", redirectToHomeConnect).Methods("GET")
	r.HandleFunc("/homeappliances/{.*}/images/{.*}", redirectToHomeConnect).Methods("GET")

	// settings
	r.HandleFunc("/homeappliances/{.*}/settings", redirectToHomeConnect).Methods("GET")
	r.HandleFunc("/homeappliances/{.*}/settings/{.*}", redirectToHomeConnect).Methods("GET", "PUT")

	// commands
	r.HandleFunc("/homeappliances/{.*}/commands", redirectToHomeConnect).Methods("GET")
	r.HandleFunc("/homeappliances/{.*}/commands/{.*}", redirectToHomeConnect).Methods("PUT")

	getAllEndpoints(*r)

	http.Handle("/", r)
	logger.Info("Home Connect client started at http://localhost:{port}", "port", port)
	http.ListenAndServe(":"+port, nil)
	return
}

// Collect all endpoints into the routes global variable. This will be served upon accessing the '/'
func getAllEndpoints(r mux.Router) {
	routes = "available endpoints: \r\n"
	r.Walk(func(route *mux.Route, router *mux.Router, ancestors []*mux.Route) error {
		t, err := route.GetPathTemplate()
		if err != nil {
			return err
		}
		routes += "\t" + t + "\r\n"
		return nil
	})
}

// Serve the available endpoints upon request to '/'
func homePageHandler(w http.ResponseWriter, r *http.Request) {
	resp := []byte(routes)
	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/text")
	w.Write(resp)
	return
}

// This handler is used to trigger the authorization flow
func authPageHandler(w http.ResponseWriter, r *http.Request) {

	// w.Header().Set("Content-Type", "application/html")
	// Render a template with our page data

	auth_uri, err := authUriTemplate(clientData)

	// If we got an error, write it out and exit
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(err.Error()))
		return
	}

	http.Redirect(w, r, auth_uri, http.StatusMovedPermanently)

	return
}

// Compiles the authourization request link using supplied client data (Client ID and Scopes)
func authUriTemplate(clientData ClientData) (string, error) {
	// Define a basic text template
	auth_uri := "https://api.home-connect.com/security/oauth/authorize?client_id={{.ClientId}}&response_type=code&scope={{.ClientScopes}}"

	// Parse the template
	tmpl, err := template.New("auth").Parse(auth_uri)
	if err != nil {
		logger.Error("Error parsing auth page template: {error}", "error", err)
		return "", err
	}

	// We need somewhere to write the executed template to
	var out bytes.Buffer

	// Render the template with the data we passed in
	if err := tmpl.Execute(&out, clientData); err != nil {
		logger.Error("Error rendering auth page template: {error}", "error", err)
		return "", err
	}

	// Return the template
	return out.String(), nil
}

// return success message upon successful authentication
func authSuccessPageHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/text")
	resp := []byte("authorization completed")
	w.Write(resp)
	return
}

// redirect the requests to the Home Connect API
func redirectToHomeConnect(w http.ResponseWriter, r *http.Request) {
	resp, err := apiRequest(w, r)
	renderResult(w, resp, err)
}

// Render the returned by Home Connect JSON
func renderResult(w http.ResponseWriter, response *http.Response, err error) {
	if err != nil {
		return
	}
	defer response.Body.Close()
	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/text")
	w.Write(body)
	return
}

// Wrapper function to make API requests to Home Connect
func apiRequest(w http.ResponseWriter, r *http.Request) (response *http.Response, err error) {
	endpoint := r.URL.Path
	method := r.Method
	logger.Info("'{method}' request to '{endpoint}' received", "method", method, "endpoint", endpoint)
	var client = &http.Client{
		Timeout: time.Second * 10,
	}

	request, err := http.NewRequest(r.Method, BaseURL+endpoint, r.Body)
	if err != nil {
		return
	}

	err = setHeader(w, r, request)
	if err != nil {
		err_descr := "unable to set header for '" + endpoint + "': " + err.Error()
		logger.Error(err_descr)
		err = errors.New(err_descr)
		return
	}
	response, err = client.Do(request)
	return
}

// Handle the redirect url during the application authorization flow
func redirectHandler(w http.ResponseWriter, r *http.Request) {
	m, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		logger.Error("Redirect Error: {query} {error}", "error", r.URL.RawQuery, err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	code := m.Get("code")

	var token Token
	err = getToken(w, "AUTHORIZE", code, &token)
	if err != nil {
		http.Error(w, "Error geting token: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// fmt.Fprintln(w, "Code: ", code, " Scope: ", scope)
	http.Redirect(w, r, "../success", http.StatusTemporaryRedirect)
}

// Serialize token to disk for access when application is restarted, and for using the refresh token
func cacheToken(token Token) (err error) {

	data, err := json.Marshal(token)
	if err != nil {
		logger.Error("Error encoding token json to save to disk: {err}", "err", err.Error())
		return
	}

	err = ioutil.WriteFile("token.cache", data, 0644)
	if err != nil {
		logger.Error("Error saving token to disk: {err}", "err", err.Error())
		return
	}
	return
}

// Load the cached token file to use its refresh token
func loadToken() (token Token, err error) {
	file, err := ioutil.ReadFile("token.cache")
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

// Get initial auth token, or refresh it using refresh token from cache
func getToken(w http.ResponseWriter, requestType string, code string, token *Token) (err error) {

	logger.Info("Getting a new '{token}' token for API access ...", "token", requestType)
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
		err_descr := "Error saving token data" + err.Error()
		logger.Error(err_descr)
		err = errors.New(err_descr)
		return
	}

	logger.Info("Completed getToken request for '{request_type}'", "request_type", requestType)
	return
}

//get Access Token, refresh if expired, set header bearer token
func setHeader(w http.ResponseWriter, r *http.Request, newReq *http.Request) (err error) {

	token, err := loadToken()
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
		err = getToken(w, "REFRESH", token.RefreshToken, &newToken)
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
