package mqttpublisher

import (
	"bufio"
	"bytes"
	"fmt"
	"net/http"
	"time"
	// "strings"
)

const (
	// SSE name constants
	eventName = "event"
	dataName  = "data"
	eqidName  = "id"

	// this is the proxied Home Connect sse stream endpoint for all devices
	sseEndpoint = "/homeappliances/events"
)

// Event is the go representation of Home Connect server-sent event
type Event struct {
	EventData struct {
		Equipment string
		Event     string
		Data      string
	}
	Action  string
	Message string
}

var (
	// ErrNilChan will be returned by Notify if it is passed a nil channel
	ErrNilChan = fmt.Errorf("nil channel given")

	// Client is the default client used for requests.
	Client = &http.Client{}

	// Global variable to store the sse stream url
	uri string
)

// URL is used to initialise the sse client by building the full SSE stream url
func InitSSEClient(port string) {
	uri = "http://localhost:" + port + sseEndpoint
	return
}

func Notify(evCh chan<- Event) {
	if evCh == nil {
		writeMessage(ErrNilChan.Error(), "error", evCh)
		return
	}

	req, err := http.NewRequest("GET", uri, nil)
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Transfer-Encoding", "chunked")
	if err != nil {
		msg := "Error in sse request: '" + err.Error() + "'"
		writeMessage(msg, "error", evCh)
		return
	}

	// wait a sec for the proxy to be up and running before subscribing to its SSE stream
	time.Sleep(1 * time.Second)
	result, err := Client.Do(req)

	if err != nil {
		msg := "Error performing sse GET request: '" + err.Error() + "'"
		writeMessage(msg, "error", evCh)
		return
	}
	defer result.Body.Close()

	scanner := bufio.NewScanner(result.Body)

	var evnt []string
	for scanner.Scan() {
		text := scanner.Text()
		// events in the stream are separated by empty line
		if len(text) > 0 {
			evnt = append(evnt, text)
		} else { // empty line separator reached -> full event data read
			writeEvent(evnt, evCh)
			evnt = nil
		}
	}

	if scanner.Err() != nil {
		writeMessage("Error reading SSE stream", "error", evCh)
		//logger.Error("Error reading SSE stream: '{e}'", "e", scanner.Err().Error())
	} else {
		//logger.Error("io.EOF reached")
		writeMessage("io.EOF reached ...", "reconnect", evCh)
	}
}

func writeMessage(err string, action string, evCh chan<- Event) {
	event := Event{}
	event.Message = err
	event.Action = action
	evCh <- event
}

// Publish an Event down the channel upon validating
// it is not of 'keep alive' type
func writeEvent(evntSlice []string, evCh chan<- Event) {
	if len(evntSlice) < 3 {
		// non 'keep alive' events would always contain 3 lines for resp. event type, event data and equipment id
		return
	}
	event := Event{}

	for i := range evntSlice {
		b := []byte(evntSlice[i])
		// split by the first occurence of ':'
		spl := bytes.SplitN(b, []byte{':'}, 2)

		switch string(spl[0]) {
		case eventName:
			event.EventData.Event = string(spl[1])
		case dataName:
			event.EventData.Data = string(spl[1])
		case eqidName:
			event.EventData.Equipment = string(spl[1])
		}
	}
	// write event to the channel
	evCh <- event
}
