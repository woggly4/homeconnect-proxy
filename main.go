package main

import (
	"fmt"
	"os"

	"github.com/ananchev/homeconnect-proxy/internal/logger"
	"github.com/ananchev/homeconnect-proxy/internal/mqttpublisher"
	"github.com/ananchev/homeconnect-proxy/internal/proxy"

	"github.com/ilyakaznacheev/cleanenv"
)

// Config is the application configuration structure
type Config struct {
	OAuth struct {
		ClientID     string `env:"CLIENT_ID" env-description:"Home Connect application client ID"`
		ClientSecret string `env:"CLIENT_SECRET" env-description:"Home Connect application client secret"`
		ClientScopes string `env:"CLIENT_SCOPES" env-description:"Home Connect application authorization scopes"`
	}

	Server struct {
		Host string `env:"HOST" env-description:"Server host" env-default:"localhost"`
		Port string `env:"PORT" env-description:"Server port" env-default:"8088"`
	}

	MQTT struct {
		Host  string `env:"MQTT_HOST" env-description:"MQTT Server host" env-default:"localhost"`
		Port  string `env:"MQTT_PORT" env-description:"MQTT Server port" env-default:"1883"`
		Topic string `env:"MQTT_TOPIC" env-description:"MQTT Topic under which to publish event data" env-default:"hc-proxy"`
	}
}

func main() {
	var cfg Config

	// read configuration from environment variables
	if err := cleanenv.ReadEnv(&cfg); err != nil {
		fmt.Println(err)
		os.Exit(2)
	}

	logger.Info("Starting the Home Connect client proxy ...")
	go proxy.Run(cfg.Server.Port, cfg.OAuth.ClientID, cfg.OAuth.ClientSecret, cfg.OAuth.ClientScopes)

	logger.Info("Starting the MQTT publisher for received SSE events ...")
	mqttpublisher.InitSSEClient(cfg.Server.Port)
	mqttpublisher.InitMqttPublisher(cfg.MQTT.Host, cfg.MQTT.Port, cfg.MQTT.Topic)

	events := make(chan mqttpublisher.Event)

	go mqttpublisher.Notify(events)

	for evnt := range events {
		go mqttpublisher.Publish(evnt)
	}

}
