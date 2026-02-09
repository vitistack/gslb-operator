package failover

import "net/http"

type FailoverService struct {
}

func NewFailoverService() *FailoverService {
	return &FailoverService{}
}

func (fs *FailoverService) FailoverService(w http.ResponseWriter, r *http.Request) {

}
