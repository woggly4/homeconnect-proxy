package mqttpublisher

import (
	"bufio"
	"bytes"
	"fmt"
	"net/http"

	// "strings"

	"github.com/ananchev/homeconnect-proxy/internal/logger"
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
	EquipmentID string
	EventName   string
	EventData   string
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

//Notify will send an Event down the channel when recieved
//This is blocking, and so you will likely want to call this
//in a new goroutine (via `go Notify(..)`)
func Notify(evCh chan<- Event) {
	if evCh == nil {
		logger.Error(ErrNilChan.Error())
		return
	}

	req, err := http.NewRequest("GET", uri, nil)
	req.Header.Set("Accept", "text/event-stream")
	if err != nil {
		logger.Error("error in sse request: '{e}'", "e", err)
		return
	}

	result, err := Client.Do(req)
	if err != nil {
		logger.Error("error performing sse GET request: '{e}'", "e", err)
		return
	}

	// bodyReader := bufio.NewReader(result.Body)
	bodyReader := bufio.NewReader(result.Body)
	defer result.Body.Close()

	delim := []byte{':'}

	// currEvent := &Event{}
	currEvent := Event{}

	for {
		bs, _, _ := bodyReader.ReadLine()

		// eliminate the empty line separators
		if len(bs) < 2 {
			continue
		}

		// split by the first occurence of ':'
		spl := bytes.SplitN(bs, delim, 2)

		switch string(spl[0]) {
		case eventName:
			currEvent.EventName = string(spl[1])
		case dataName:
			currEvent.EventData = string(spl[1])
		case eqidName:
			currEvent.EquipmentID = string(spl[1])
			// write event to the channel only if id is found at the end
			evCh <- currEvent
		}
	}

}
