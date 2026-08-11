package spoof

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/vitistack/gslb-operator/internal/config"
	"github.com/vitistack/gslb-operator/internal/model"
	"github.com/vitistack/gslb-operator/pkg/iter"
	spoofModel "github.com/vitistack/gslb-operator/pkg/models/spoofs"
	"github.com/vitistack/gslb-operator/pkg/persistence"
)

var (
	ErrSpoofInServiceGroupNotFound = errors.New("spoof in service group not found")
)

// read-only repo for spoofModel.Spoof
type SpoofRepo struct {
	store persistence.Store[model.GSLBServiceGroup]
}

func NewSpoofRepo(storage persistence.Store[model.GSLBServiceGroup]) *SpoofRepo {
	return &SpoofRepo{
		store: storage,
	}
}

func (r *SpoofRepo) Read(memberOf string, views ...string) (spoofModel.Spoof, error) {
	group, err := r.store.Load(memberOf)
	if err != nil {
		return spoofModel.Spoof{}, fmt.Errorf("failed to read from storage: %w", err)
	}

	spoof := group.Spoof(views...)
	if spoof == nil {
		return spoofModel.Spoof{}, fmt.Errorf("no active spoof for %s", memberOf)
	}

	return *spoof, nil
}

func (r *SpoofRepo) ReadAll() (iter.Iterator[spoofModel.Spoof], func() error) {
	groups, finish := r.store.LoadAll()

	var iterError error
	seq := func(yield func(spoofModel.Spoof) bool) {
		for group := range groups {
			for _, view := range group.Views {
				spoof := group.Spoof(view)
				if spoof != nil && spoof.Address != nil {
					if !yield(*spoof) {
						return
					}
					continue
				}
			}
		}

		iterError = finish()
	}

	finish = func() error {
		if iterError != nil {
			return fmt.Errorf("failed to read from storage: %w", iterError)
		}
		return nil
	}

	return seq, finish
}

// hash of all the combined uuids from service groups
func (r *SpoofRepo) Hash(views ...string) (string, error) {
	ids := make([]string, 0)

	if len(views) == 0 {
		views = []string{
			config.SplitDNS().DefaultView(),
		}
	}

	spoofs, finish := r.ReadAll()
	spoofs.Filter(
		func(s spoofModel.Spoof) bool { return slices.Contains(views, s.View) },
	).Each(
		func(s spoofModel.Spoof) { ids = append(ids, s.UUID) },
	)

	if err := finish(); err != nil {
		return "", fmt.Errorf("failed to produce spoof-hash: %w", err)
	}

	slices.Sort(ids)

	joinedIDs := strings.Join(ids, ",")
	rawHash := sha256.Sum256([]byte(joinedIDs))

	return hex.EncodeToString(rawHash[:]), nil
}
