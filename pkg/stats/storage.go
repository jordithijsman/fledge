package stats

import (
	"gitlab.ilabt.imec.be/fledge/service/pkg/util"
	"k8s.io/apimachinery/pkg/api/resource"
	"strings"
)

func StorageSize() (resource.Quantity, error) {
	// TODO: Detect which disk is used for storage and use that folder
	storSize, _ := util.ExecShellCommand("findmnt / --output=SIZE --noheadings --raw")
	return resource.ParseQuantity(strings.TrimSpace(storSize))
}

func StorageUsed() (resource.Quantity, error) {
	// TODO: Detect which disk is used for storage and use that folder
	storUsed, _ := util.ExecShellCommand("findmnt / --output=USED --noheadings --raw")
	return resource.ParseQuantity(strings.TrimSpace(storUsed))
}

func StorageAvailable() (resource.Quantity, error) {
	// TODO: Detect which disk is used for storage
	storAvailable, _ := util.ExecShellCommand("findmnt / --output=AVAIL --noheadings --raw")
	return resource.ParseQuantity(strings.TrimSpace(storAvailable))
}

func IsStoragePressure() bool {
	storTotal, _ := StorageSize()
	storAvailable, _ := StorageAvailable()
	return (storAvailable.AsApproximateFloat64() / storTotal.AsApproximateFloat64()) <= 0.1
}

func IsStorageFull() bool {
	storTotal, _ := StorageSize()
	storAvailable, _ := StorageAvailable()
	return (storAvailable.AsApproximateFloat64() / storTotal.AsApproximateFloat64()) <= 0.01
}
