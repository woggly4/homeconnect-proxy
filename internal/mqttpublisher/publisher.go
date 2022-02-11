package mqttpublisher

import (
	"github.com/ananchev/homeconnect-proxy/internal/logger"
	mqtt "github.com/eclipse/paho.mqtt.golang"
)

//store here publisher params
var RootTopic string
var Server string
var Port string

//Create the ClientOptions struct
func InitMqttPublisher(server string, port string, rootTopic string) {
	RootTopic = rootTopic
	Server = server
	Port = port
	logger.Info("Initialized MQTT publisher for broker '{b}' and root topic '{rt}'", "b", server+":"+port, "rt", rootTopic)
}

func Publish(ev Event) {

	opts := mqtt.NewClientOptions()
	opts.AddBroker("tcp://" + Server + ":" + Port)
	opts.SetClientID("homeconnect-proxy")

	opts.OnConnectionLost = func(c mqtt.Client, e error) {
		logger.Error("Connection to mqtt server lost: '{error}'", "error", e.Error())
	}

	topic := RootTopic + "/" + ev.EventData.Equipment + "/" + ev.EventData.Event
	payload := ev.EventData.Data

	client := mqtt.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		logger.Error("Error in MQTT client connection: '{err}'", "err", token.Error())
		return
	}
	logger.Info("Publishing event '{evnt}' for equipment '{eq}'", "evnt", ev.EventData.Event, "eq", ev.EventData.Equipment)
	token := client.Publish(topic, 0, false, payload)
	token.Wait()

	client.Disconnect(250)
}
