package stats

import (
	"gitlab.ilabt.imec.be/fledge/service/pkg/util"
	"k8s.io/apimachinery/pkg/api/resource"
	"strings"
)

func MemoryTotal() (resource.Quantity, error) {
	memTotal, _ := util.ExecShellCommand("cat /proc/meminfo | grep -oP '^MemTotal: +\\K.+$'")
	return resource.ParseQuantity(strings.TrimSpace(memTotal))
}

func MemoryFree() (resource.Quantity, error) {
	memFree, _ := util.ExecShellCommand("cat /proc/meminfo | grep -oP '^MemFree: +\\K.+$'")
	return resource.ParseQuantity(strings.TrimSpace(memFree))
}

func MemoryAvailable() (resource.Quantity, error) {
	memAvailable, _ := util.ExecShellCommand("cat /proc/meminfo | grep -oP '^MemAvailable: +\\K.+$'")
	return resource.ParseQuantity(strings.TrimSpace(memAvailable))
}

func IsMemoryPressure() bool {
	memTotal, _ := MemoryTotal()
	memAvailable, _ := MemoryAvailable()
	return (memAvailable.AsApproximateFloat64() / memTotal.AsApproximateFloat64()) <= 0.1
}
