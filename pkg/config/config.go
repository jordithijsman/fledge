package config

import (
	"encoding/json"
	"gopkg.in/validator.v2"
	"io"
	"k8s.io/klog/v2"
	"os"
	"reflect"
	"strconv"
	"strings"
)

type Config struct {
	Runtime         string `json:"runtime" env:"RUNTIME"`
	DeviceName      string `json:"deviceName" env:"DEVICE_NAME"`
	ShortDeviceName string `json:"shortDeviceName"`
	DeviceIP        string `json:"deviceIP" env:"DEVICE_IP"`
	ServicePort     string `json:"servicePort" env:"SERVICE_PORT"`
	KubeletPort     string `json:"kubeletPort" env:"KUBELET_PORT"`
	VKubeServiceURL string `json:"vKubeServiceURL" env:"VKUBE_URL"`
	UseKubeAPI      bool   `json:"useKubeAPI"`
	FledgeAPIPort   int    `json:"fledgeAPIPort"`
	IgnoreKubeProxy string `json:"ignoreKubeProxy" env:"IGNORE_KPROXY"`
	Interface       string `json:"interface" env:"INET_INTERFACE"`
	HeartbeatTime   int    `json:"heartbeatTime" env:"HEARTBEAT_TIME" validate:"min=1"`
}

func Load(filename string) *Config {
	klog.V(1).Infof("Loading config from %s..\n", filename)
	cfg := &Config{}

	// Open file
	file, err := os.Open(filename)
	defer file.Close()

	// Fallback to an empty json if the file does not exist
	var reader io.Reader
	if err == nil {
		reader = file
	} else if os.IsNotExist(err) {
		klog.V(1).Infof("'%s' does not exist\n", filename)
		reader = strings.NewReader("{}")
	} else if err != nil {
		klog.Fatalln(err)
	}

	// Parse file as JSON
	if err = json.NewDecoder(reader).Decode(&cfg); err != nil {
		klog.Fatalln(err)
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
						klog.Fatalln(err)
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
		klog.Fatalf("Config is invalid (%s)", err)
	}
	klog.Infof("Config is valid %+v\n", cfg)
	return cfg
}
