package config

import (
	"context"
	"encoding/json"
	"github.com/virtual-kubelet/virtual-kubelet/log"
	"gopkg.in/validator.v2"
	"io"
	"os"
	"reflect"
	"strconv"
	"strings"
)

type Config struct {
	Runtime         string `json:"runtime" env:"RUNTIME" validate:"nonzero"`
	DeviceName      string `json:"deviceName" env:"DEVICE_NAME"`
	ShortDeviceName string `json:"shortDeviceName"`
	DeviceIP        string `json:"deviceIP" env:"DEVICE_IP"`
	ServicePort     int    `json:"servicePort" env:"SERVICE_PORT" validate:"nonzero"`
	KubeletPort     int    `json:"kubeletPort" env:"KUBELET_PORT" validate:"nonzero"`
	VKubeServiceURL string `json:"vKubeServiceURL" env:"VKUBE_URL"`
	UseKubeAPI      bool   `json:"useKubeAPI"`
	FledgeAPIPort   int    `json:"fledgeAPIPort"`
	IgnoreKubeProxy string `json:"ignoreKubeProxy" env:"IGNORE_KPROXY"`
	Interface       string `json:"interface" env:"INET_INTERFACE"`
	HeartbeatTime   int    `json:"heartbeatTime" env:"HEARTBEAT_TIME" validate:"min=1"`
	// API server options
	CertPath   string `json:"certPath" env:"CERT_PATH" validate:"nonzero"`
	KeyPath    string `json:"keyPath" env:"KEY_PATH" validate:"nonzero"`
	CACertPath string `json:"caCertPath" env:"CA_CERT_PATH" validate:"nonzero"`
	// Features
	EnableMetrics bool `json:"metrics" env:"METRICS"`
}

func LoadConfig(ctx context.Context, path string) (*Config, error) {
	log.G(ctx).Debugf("Loading config from %s..\n", path)
	cfg := &Config{}

	// Open file
	file, err := os.Open(path)
	defer file.Close()

	// Fallback to an empty json if the file does not exist
	var reader io.Reader
	if err == nil {
		reader = file
	} else if os.IsNotExist(err) {
		log.G(ctx).Debugf("'%s' does not exist\n", path)
		reader = strings.NewReader("{}")
	} else if err != nil {
		return nil, err
	}

	// Parse file as JSON
	if err = json.NewDecoder(reader).Decode(&cfg); err != nil {
		return nil, err
	}

	// Override config with env vars
	t := reflect.TypeOf(*cfg)
	v := reflect.ValueOf(cfg)
	for i := 0; i < t.NumField(); i++ {
		tf := t.Field(i)
		vf := v.Elem().Field(i)
		if tag := tf.Tag.Get("env"); tag != "" {
			if val := os.Getenv("FLEDGE_" + tag); val != "" {
				if vf.Kind() == reflect.Int {
					intVal, err := strconv.Atoi(val)
					if err != nil {
						return nil, err
					}
					vf.SetInt(int64(intVal))
				} else if vf.Kind() == reflect.String {
					vf.SetString(val)
				}
			}
		}
	}
	// Validate configuration
	if err := validator.Validate(cfg); err != nil {
		return nil, err
	}
	log.G(ctx).Debugf("Config is valid %+v\n", cfg)
	return cfg, nil
}
