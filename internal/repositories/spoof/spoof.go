package spoof

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/vitistack/gslb-operator/internal/model"
	"github.com/vitistack/gslb-operator/pkg/models/spoofs"
	"github.com/vitistack/gslb-operator/pkg/persistence"
)

var (
	ErrSpoofInServiceGroupNotFound = errors.New("spoof in service group not found")
)

// read-only repo for spoofs
type SpoofRepo struct {
	store persistence.Store[model.GSLBServiceGroup]
}

func NewSpoofRepo(storage persistence.Store[model.GSLBServiceGroup]) *SpoofRepo {
	return &SpoofRepo{
		store: storage,
	}
}

func (r *SpoofRepo) Read(memberOf string) (spoofs.Spoof, error) {
	group, err := r.store.Load(memberOf)
	if err != nil {
		return spoofs.Spoof{}, fmt.Errorf("failed to read from storage: %w", err)
	}

	spoof := group.Spoof()
	if spoof == nil {
		return spoofs.Spoof{}, fmt.Errorf("no active spoof for %s", memberOf)
	}

	return *spoof, nil
}

func (r *SpoofRepo) ReadAll() ([]spoofs.Spoof, error) {
	groups, err := r.store.LoadAll()
	if err != nil {
		return nil, fmt.Errorf("failed to read from storage: %w", err)
	}

	result := make([]spoofs.Spoof, 0)
	for _, group := range groups {
		spoof := group.Spoof()
		if spoof != nil && spoof.Address != nil {
			result = append(result, *spoof)
		}
	}

	return result, nil
}

// hash of all the combined uuids from service groups
func (r *SpoofRepo) Hash() (string, error) {
	groups, err := r.store.LoadAll()
	if err != nil {
		return "", fmt.Errorf("failed to read from storage: %w", err)
	}

	ids := make([]string, 0, len(groups))
	for _, group := range groups {
		ids = append(ids, group.UUID.String())
	}
	slices.Sort(ids)

	joinedIDs := strings.Join(ids, ",")
	rawHash := sha256.Sum256([]byte(joinedIDs))

	return hex.EncodeToString(rawHash[:]), nil
}
