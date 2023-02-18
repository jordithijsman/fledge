package main

import (
	"context"
	"fmt"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"
	"gitlab.ilabt.imec.be/fledge/service/cmd/fledge/internal/commands/root"
	"gitlab.ilabt.imec.be/fledge/service/cmd/fledge/internal/provider"
	"gitlab.ilabt.imec.be/fledge/service/pkg/config"
	"gitlab.ilabt.imec.be/fledge/service/pkg/util"
	"os"
	"regexp"
	"runtime"
	"strconv"
	"strings"
)

// / Patch virtual-kubelet without modifying the sources too much
func patchCmd(ctx context.Context, rootCmd *cobra.Command, s *provider.Store, c root.Opts) {
	var configPath string
	rootCmd.PersistentFlags().StringVar(&configPath, "config-path", "default.json", "set the config path")
	rootCmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		if configPath != "" {
			cfg, err := config.LoadConfig(ctx, configPath)
			if err != nil {
				return errors.Wrap(err, "could not parse config")
			}
			// Patch options
			patchOpts(cfg, cmd, c)
			// Configure the appropriate runtime
			switch cfg.Runtime {
			case "containerd":
				// cri := (&fledge.ContainerdRuntimeInterface{}).Init()
			default:
				return errors.New(fmt.Sprintf("runtime '%s' is not supported\n", cfg.Runtime))
			}
		}
		return nil
	}
}

func patchOpts(cfg *config.Config, cmd *cobra.Command, c root.Opts) {
	// Set default commandline arguments
	patchOpt(cmd.Flags(), "nodename", cfg.DeviceName)
	patchOpt(cmd.Flags(), "os", runtime.GOOS)
	patchOpt(cmd.Flags(), "provider", cfg.Runtime)
	patchOpt(cmd.Flags(), "provider-config", cfg.RuntimeConfig)
	patchOpt(cmd.Flags(), "pod-sync-workers", strconv.FormatInt(int64(runtime.NumCPU()), 10))
	patchOpt(cmd.Flags(), "enable-node-lease", strconv.FormatBool(true))

	// Set kubernetes version
	k8sVersion, _ := util.ReadDepVersion("k8s.io/api")
	k8sVersion = regexp.MustCompile("^v0").ReplaceAllString(k8sVersion, "v1")
	c.Version = strings.Join([]string{k8sVersion, "fledge", buildVersion}, "-")

	// Populate apiserver options
	os.Setenv("APISERVER_CERT_LOCATION", cfg.CertPath)
	os.Setenv("APISERVER_KEY_LOCATION", cfg.KeyPath)
	os.Setenv("APISERVER_CA_CERT_LOCATION", cfg.CACertPath)
}

func patchOpt(flags *flag.FlagSet, name string, value string) {
	f := flags.Lookup(name)
	if !f.Changed {
		f.Value.Set(value)
	}
}
