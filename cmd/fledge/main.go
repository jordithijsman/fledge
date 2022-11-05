package main

import (
	"context"
	"flag"
	"github.com/sirupsen/logrus"
	"github.com/virtual-kubelet/virtual-kubelet/log"
	logruslogger "github.com/virtual-kubelet/virtual-kubelet/log/logrus"
	"github.com/virtual-kubelet/virtual-kubelet/trace"
	"github.com/virtual-kubelet/virtual-kubelet/trace/opencensus"
	"gitlab.ilabt.imec.be/fledge/service/pkg/config"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sig
		cancel()
	}()
	log.L = logruslogger.FromLogrus(logrus.NewEntry(logrus.StandardLogger()))
	trace.T = opencensus.Adapter{}

	// Parse arguments
	cfgFilename := flag.String("config", "default.json", "<config>")
	enableDebug := flag.Bool("debug", false, "<debug>")
	flag.Parse()

	// Enable debugging
	if *enableDebug {
		logrus.SetLevel(logrus.DebugLevel)
	}

	// Load config file
	cfg := config.Load(*cfgFilename)

	// Configure the appropriate runtime
	switch cfg.Runtime {
	case "containerd":
		// cri := (&vkube.ContainerdRuntimeInterface{}).Init()
	default:
		log.G(ctx).Fatalf("Container runtime '%s' is not supported\n", cfg.Runtime)
	}

	if err := Start(ctx, cfg); err != nil {
		log.G(ctx).Fatalf("Starting failed (%s)", err)
	}
}
