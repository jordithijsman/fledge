package stats

import (
	"gitlab.ilabt.imec.be/fledge/service/pkg/util"
	"k8s.io/apimachinery/pkg/api/resource"
	"strings"
)

func CpuCount() (resource.Quantity, error) {
	cpuCount, _ := util.ExecShellCommand("nproc")
	return resource.ParseQuantity(strings.TrimSpace(cpuCount))
}
