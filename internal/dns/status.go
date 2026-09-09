package dns

import (
	"uuid"

	"github.com/vitistack/gslb-operator/pkg/models/service"
)

type StatusFetcher interface {
	FetchStatus(id string) (service.DNSStatusForService, error)
	BulkFetchStatus() (map[uuid.UUID]service.DNSStatusForService, error)
}
