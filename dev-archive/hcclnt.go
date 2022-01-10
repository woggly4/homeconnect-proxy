package hcclnt

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	//"fmt"
	"html/template"
	"io/ioutil"
	"net/http"
	"net/url"

	//"os"
	"path"
	"strings"
	"time"

	"github.com/ananchev/homeconnect-proxy/internal/logger"
	"github.com/gorilla/mux"
)

const (
	ClientID     = "64948DF768211857B2D01C09D17A020B342230A1930183B127787B38DCA426A0"
	ClientSecret = "691A0CD32054AF94D76C8DBD6C390ED06CAF8D143C08DACA88D4C473AB2D8D8E"
	TokenURL     = "https://api.home-connect.com/security/oauth/token"
	CookieName   = "HomeConnect_Oauth2.cookie"
	BaseURL      = "https://api.home-connect.com/api"
)

var routes string
var applianceDefs map[string]string

// Documentation: https://api-docs.home-connect.com/authorization
// Authorization URL: https://api.home-connect.com/security/oauth/authorize
// Token URL: https://api.home-connect.com/security/oauth/token

func Initiate(port string, applianceDefinitions map[string]string) {
	applianceDefs = applianceDefinitions

	r := mux.NewRouter()
	r.HandleFunc("/", homePageHandler)
	r.HandleFunc("/auth", authPageHandler)
	r.HandleFunc("/oauth/redirect", redirectHandler)
	r.HandleFunc("/success", authSuccessPageHandler)
	r.HandleFunc("/homeappliances", getHandler)
	r.HandleFunc("/homeappliances/{appliance}/programs", getHandler)
	r.HandleFunc("/homeappliances/{appliance}/programs/active", putHandler).Methods("PUT")

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

// Serve auth.html upon request to '/auth'. This is used to trigger the authorization flow
func authPageHandler(w http.ResponseWriter, r *http.Request) {
	pageHandler(w, r, nil, "web", "auth.html")
}

// return success message upon successful authentication
func authSuccessPageHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/text")
	resp := []byte("authorization completed")
	w.Write(resp)
	return
}

// handle all PUT requests to the Home Connect API
func putHandler(w http.ResponseWriter, r *http.Request) {
	logger.Info("I am a PUT request")

}

// handle all GET requests to the Home Connect API
func getHandler(w http.ResponseWriter, r *http.Request) {
	uri := r.URL.Path
	vars := mux.Vars(r)

	// check if the uri contains the appliance variable
	if val, appliance_exists := vars["appliance"]; appliance_exists {

		// is the equipment managed the proxy?
		if _, haId_exists := applianceDefs[val]; !haId_exists {
			w.WriteHeader(http.StatusOK)
			w.Header().Set("Content-Type", "application/text")
			resp_str := fmt.Sprintf("Equipment '%s' is not managed by the proxy.", val)
			resp := []byte(resp_str)
			w.Write(resp)
			return
		}

		// appliance variable found -> replace it with the haId
		replace_args := make([]string, 2)
		replace_args = append(replace_args, val)
		replace_args = append(replace_args, applianceDefs[val])
		replacer := strings.NewReplacer(replace_args...)
		uri = replacer.Replace(uri)
	}

	// Replace any appliance name in the uri with the corresponding haId

	resp, err := apiRequest(w, r, "GET", uri)
	renderGetResult(w, resp, err)
}

// Render the returned by Home Connect JSON
func renderGetResult(w http.ResponseWriter, response *http.Response, err error) {
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

// Generic function to make API requests to Home Connect
func apiRequest(w http.ResponseWriter, r *http.Request, method, endpoint string) (response *http.Response, err error) {
	logger.Info("'{endpoint}' requested", "endpoint", endpoint)
	var client = &http.Client{
		Timeout: time.Second * 10,
	}
	var body io.Reader

	request, err := http.NewRequest(method, BaseURL+endpoint, body)
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

	var token string
	err = getToken(w, "AUTHORIZE", code, &token)
	if err != nil {
		http.Error(w, "Error geting token: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// fmt.Fprintln(w, "Code: ", code, " Scope: ", scope)
	http.Redirect(w, r, "../success", http.StatusTemporaryRedirect)
}

// Generic function used to serve HTML pages
func pageHandler(w http.ResponseWriter, r *http.Request, data interface{}, dir string, filenames ...string) {
	var files []string
	for _, file := range filenames {
		files = append(files, path.Join(dir, file))
	}
	tmpl, err := template.ParseFiles(files...)
	if err != nil {
		logger.Error("Internal server error: {err}", "err", err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := tmpl.Execute(w, data); err != nil {
		logger.Error("Internal server error: {err}", "err", err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

// setCookie is a generic cookie setter to store token data.
func setCookie(w http.ResponseWriter, token string) {
	tok64 := base64.StdEncoding.EncodeToString([]byte(token))
	cookie := http.Cookie{
		Name:     CookieName,
		Value:    tok64,
		HttpOnly: true,
		Secure:   false, //use true for production
		Path:     "/",
		SameSite: http.SameSiteLaxMode,
	}
	http.SetCookie(w, &cookie)
	logger.Info("token cookie '{cookie_name}' updated.", "cookie_name", CookieName)
}

// getCookie is a generic cookie getter to obtain cached token data.
func getCookie(r *http.Request) (token string, err error) {
	logger.Info("Get cached token data from cookie...")
	cookie, err := r.Cookie(CookieName)
	if err != nil {
		return
	}
	tokb, err := base64.StdEncoding.DecodeString(cookie.Value)
	if err != nil {
		return
	}
	token = string(tokb)
	return
}

// Get initial auth token, or refresh it using refresh token from cookie
func getToken(w http.ResponseWriter, requestType string, code string, tokenRetval *string) (err error) {

	logger.Info("Getting a new '{token}' token for API access ...", "token", requestType)
	// initate the payload values map, will add all values in the switch below, depending if requesting a new token, or refreshing it
	values := url.Values{}
	values.Set("client_secret", ClientSecret)

	switch requestType {
	case "AUTHORIZE":
		values.Set("client_id", ClientID)
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

	var tokenMap map[string]interface{}
	decoder := json.NewDecoder(strings.NewReader(string(body)))
	decoder.UseNumber()
	err = decoder.Decode(&tokenMap)

	// decoder error
	if err != nil {
		err_descr := "Error decoding response: " + err.Error()
		logger.Error(err_descr)
		err = errors.New(err_descr)
		return
	}

	// oauth adds an outer "body" which must be stripped off
	_, exists := tokenMap["body"]
	if exists {
		tokenMap = tokenMap["body"].(map[string]interface{})
	}

	//i f no access token
	_, exists = tokenMap["access_token"]
	if !exists {
		err_descr := "Missing access token: " + string(body)
		logger.Error(err_descr)
		err = errors.New(err_descr)
		return
	}

	// calculate expiry
	const DELTASECS = 5 // remove 5 seconds from the exipry to account for transport delay
	var expiresIn int64
	expiry, exists := tokenMap["expires_in"]
	if !exists { //no expiration, so make it a year
		expiresIn = 31536000
	} else {
		expiresIn, err = expiry.(json.Number).Int64()
	}
	tokenMap["expires_at"] = epochSeconds() + expiresIn - DELTASECS

	retval, err := json.Marshal(tokenMap)
	if err != nil {
		err_descr := "Error encoding token into json" + err.Error()
		logger.Error(err_descr)
		err = errors.New(err_descr)
		return
	}
	tokenString := string(retval)
	setCookie(w, tokenString)

	logger.Info("Completed getToken request for '{request_type}'", "request_type", requestType)
	return
}

//get Access Token via cookie, refresh if expired, set header bearer token
func setHeader(w http.ResponseWriter, r *http.Request, newReq *http.Request) (err error) {
	token, err := getCookie(r)
	if err != nil {
		err_descr := "Error getting cookie: " + err.Error()
		logger.Error(err_descr)
		return
	}
	var tokMap map[string]interface{}

	// err = json.Unmarshal([]byte(token), &tokMap)
	// normally as above, but we want numbers as ints vs floats
	decoder := json.NewDecoder(strings.NewReader(token))
	decoder.UseNumber()
	err = decoder.Decode(&tokMap)
	if err != nil {
		return
	}
	expiresAt, err := tokMap["expires_at"].(json.Number).Int64()
	if err != nil {
		return
	}
	if epochSeconds() > expiresAt { //token has expired, refresh it
		logger.Info("Access token has expired, initiating refresh...")
		refresh, exists := tokMap["refresh_token"]
		if !exists {
			err_descr := "Refresh Token Not Found. Please re-authorize the application."
			logger.Error(err_descr)
			err = errors.New(err_descr)
			return
		}
		var newToken string
		err = getToken(w, "REFRESH", refresh.(string), &newToken)
		if err != nil {
			return
		}
		setCookie(w, newToken) //note: must set cookie before writing to responsewriter
		decoder = json.NewDecoder(strings.NewReader(newToken))
		decoder.UseNumber()
		tokMap = make(map[string]interface{})
		err = decoder.Decode(&tokMap)
		if err != nil {
			return
		}
	}
	newReq.Header.Add("Authorization", "Bearer "+tokMap["access_token"].(string))
	newReq.Header.Set("Content-Type", "application/json")
	// newReq.Header.Set("Accept", "application/json")
	return
}

func epochSeconds() int64 {
	now := time.Now()
	secs := now.Unix()
	return secs
}
