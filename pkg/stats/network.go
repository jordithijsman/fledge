package stats

import (
	"gitlab.ilabt.imec.be/fledge/service/pkg/util"
)

func HostName() string {
	hostname, _ := util.ExecShellCommand("hostname")
	return util.RemoveSpace(hostname)
}

func IsNetworkAvailable() bool {
	_, err := util.ExecShellCommand("ping -c1 1.1.1.1")
	return err == nil
}
