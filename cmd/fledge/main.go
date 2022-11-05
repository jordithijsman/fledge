package main

import (
	"flag"
	"gitlab.ilabt.imec.be/fledge/service/pkg/config"
	"k8s.io/klog/v2"
)

func main() {
	// Parse arguments
	klog.InitFlags(nil)
	cfgFilename := flag.String("config", "default.json", "<config>")
	flag.Parse()

	// Load config file
	config.Load(*cfgFilename)
}
