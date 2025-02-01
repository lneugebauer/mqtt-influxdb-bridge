package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/caarlos0/env/v11"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	"github.com/joho/godotenv"
)

type HTU21 struct {
	Temperature float64
	Humidity    float64
	DewPoint    float64
}

type Payload struct {
	Time     string
	HTU21    HTU21
	TempUnit string
}

type Message struct {
	Node    string
	Payload Payload
}

type Config struct {
	MqttHost             string `env:"MQTT_HOST" envDefault:"localhost"`
	MqttPort             int    `env:"MQTT_PORT" envDefault:"1883"`
	MqttClient           string `env:"MQTT_CLIENT" envDefault:"mqtt_influxdb_bridge"`
	MqttUser             string `env:"MQTT_USER" envDefault:""`
	MqttPassword         string `env:"MQTT_PASSWORD" envDefault:""`
	MqttTopic            string `env:"MQTT_TOPIC" envDefault:"#"`
	InfluxDBHost         string `env:"INFLUXDB_HOST" envdefault:"localhost"`
	InfluxDBPort         string `env:"INFLUXDB_PORT" envdefault:"8086"`
	InfluxDBDatabase     string `env:"INFLUXDB_DATABASE" envdefault:"telemetry"`
	InfluxDBOrganization string `env:"INFLUXDB_ORGANIZATION" envdefault:"iot"`
	InfluxDBToken        string `env:"INFLUXDB_TOKEN" envdefault:""`
	InfluxDBMeasurement  string `env:"INFLUXDB_MEASUREMENT" envdefault:"iot_data"`
}

var cfg = Config{}

func parseMqttMessage(msg mqtt.Message) (*Message, error) {
	m := &Message{}
	topics := strings.Split(msg.Topic(), "/")
	m.Node = topics[2]

	var payload Payload
	err := json.Unmarshal([]byte(msg.Payload()), &payload)
	if err != nil {
		return nil, err
	}
	m.Payload = payload

	return m, nil
}

func writeToInfluxDb(m *Message) {
	client := influxdb2.NewClient(
		cfg.InfluxDBHost+":"+cfg.InfluxDBPort,
		cfg.InfluxDBToken,
	)
	writeAPI := client.WriteAPI(cfg.InfluxDBOrganization, cfg.InfluxDBDatabase)
	tags := map[string]string{"node": m.Node, "tempUnit": m.Payload.TempUnit}
	fields := map[string]interface{}{
		"temperature": m.Payload.HTU21.Temperature,
		"humidity":    m.Payload.HTU21.Humidity,
		"dew_point":   m.Payload.HTU21.DewPoint,
	}
	p := influxdb2.NewPoint(cfg.InfluxDBMeasurement, tags, fields, time.Now())
	writeAPI.WritePoint(p)
	writeAPI.Flush()
	defer client.Close()
}

var messagePubHandler mqtt.MessageHandler = func(client mqtt.Client, msg mqtt.Message) {
	fmt.Printf("Received message: \"%s\" from topic: \"%s\"\n", msg.Payload(), msg.Topic())

	m, err := parseMqttMessage(msg)
	if err != nil {
		log.Println(err)
		return
	}
	writeToInfluxDb(m)
}

var connectHandler mqtt.OnConnectHandler = func(client mqtt.Client) {
	fmt.Println("Connected")
}

var connectLostHandler mqtt.ConnectionLostHandler = func(client mqtt.Client, err error) {
	fmt.Printf("Connect lost: %v", err)
}

func sub(client mqtt.Client) {
	token := client.Subscribe(cfg.MqttTopic, 1, nil)
	token.Wait()
	fmt.Printf("Subscribed to topic: \"%s\"\n", cfg.MqttTopic)
}

func init() {
	_ = godotenv.Load()

	err := env.Parse(&cfg)
	if err != nil {
		log.Fatalf("unable to parse ennvironment variables: %e", err)
	}
}

func main() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	opts := mqtt.NewClientOptions()
	opts.AddBroker(fmt.Sprintf("tcp://%s:%d", cfg.MqttHost, cfg.MqttPort))
	opts.SetClientID(cfg.MqttClient)
	opts.SetCleanSession(true)
	opts.SetUsername(cfg.MqttUser)
	opts.SetPassword(cfg.MqttPassword)
	opts.SetDefaultPublishHandler(messagePubHandler)
	opts.OnConnect = connectHandler
	opts.OnConnectionLost = connectLostHandler
	client := mqtt.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		panic(token.Error())
	}

	sub(client)

	<-c
}
