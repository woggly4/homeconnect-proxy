package proxy

import (
	"bytes"
	"errors"
	"text/template"
	"time"

	"io/ioutil"
	"net/http"
	"net/url"

	"github.com/ananchev/homeconnect-proxy/internal/logger"
	"github.com/gorilla/mux"
)

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

	// status
	r.HandleFunc("/homeappliances/{.*}/status", redirectToHomeConnect).Methods("GET")
	r.HandleFunc("/homeappliances/{.*}/status/{.*}", redirectToHomeConnect).Methods("GET")

	// status event streams are proxied in separate go routine
	go StartServerSentEventProxy()

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
	logger.Info("Web interface accessible at http://localhost:{port}", "port", port)

	http.ListenAndServe(":"+port, nil)
	return
}

// Collect all endpoints into the routes global variable. This will be served upon accessing the '/'
func getAllEndpoints(r mux.Router) {
	routes = "Welcome to Home Connect proxy!"
	// TODO: implement a check if there is token.cache file existing???
	routes += "If running for first time, please make sure to authorize via '/proxy/auth' before using any of the '/homeconnect' routes.\r\n"
	routes += "The available endpoints are listed below.\r\n"
	r.Walk(func(route *mux.Route, router *mux.Router, ancestors []*mux.Route) error {
		t, err := route.GetPathTemplate()
		if err != nil {
			return err
		}
		routes += "\t" + t + "\r\n"
		return nil
	})
	//manually add the additional two SSE handles for complete list
	routes += "\t" + "/homeappliances/{.*}/events" + "\r\n"
	routes += "\t" + "/homeappliances/events" + "\r\n"
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

// Creates the authourization request link using supplied client data (Client ID and Scopes)
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

// Return success message upon successful authentication
func authSuccessPageHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/text")
	resp := []byte("authorization completed")
	w.Write(resp)
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
	err = requestToken("AUTHORIZE", code, &token)
	if err != nil {
		http.Error(w, "Error geting token: "+err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "../success", http.StatusTemporaryRedirect)
}

// redirect the requests to the Home Connect API
func redirectToHomeConnect(w http.ResponseWriter, r *http.Request) {
	resp, err := apiRequest(r)
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
func apiRequest(proxyRequest *http.Request) (response *http.Response, err error) {
	endpoint := proxyRequest.URL.Path
	method := proxyRequest.Method
	logger.Info("'{method}' request to '{endpoint}' received", "method", method, "endpoint", endpoint)
	var client = &http.Client{
		Timeout: time.Second * 10,
	}

	request, err := http.NewRequest(method, BaseURL+endpoint, proxyRequest.Body)
	if err != nil {
		return
	}

	err = setHeader(request)
	if err != nil {
		err_descr := "unable to set header for '" + endpoint + "': " + err.Error()
		logger.Error(err_descr)
		err = errors.New(err_descr)
		return
	}
	response, err = client.Do(request)
	return
}
