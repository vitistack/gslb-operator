package manager

import (
    "strings"
    "sync"

    "github.com/vitistack/gslb-operator/internal/dns/update"
)

// dnsGate sits between service-group promotion and the real DNS updater: it
// remembers the last record applied per group/view and skips re-pushing one
// that already matches
type dnsGate struct {
    mu      sync.Mutex
    applied map[string]update.Record // key: uuid + "|" + view

    create func(...update.Record) error
    delete func(string, ...string) error
}

func newDNSGate(create func(...update.Record) error, del func(string, ...string) error) *dnsGate {
    return &dnsGate{
        applied: make(map[string]update.Record),
        create:  create,
        delete:  del,
    }
}

// Seed marks rec as already known-applied without touching dnsdist; used to prime
// state from persisted data before the first reconcile of a run.
func (g *dnsGate) Seed(rec update.Record) {
    g.mu.Lock()
    defer g.mu.Unlock()
    for _, view := range rec.Views {
        g.applied[rec.UUID+"|"+view] = rec
    }
}

func (g *dnsGate) Create(records ...update.Record) error {
    g.mu.Lock()
    pending := make([]update.Record, 0, len(records))
    for _, rec := range records {
        if g.changed(rec) {
            pending = append(pending, rec)
        }
    }
    g.mu.Unlock()

    if len(pending) == 0 {
        return nil // every record already matches last-known state
    }

    if err := g.create(pending...); err != nil {
        g.mu.Lock()
        for _, rec := range pending {
            for _, view := range rec.Views {
                delete(g.applied, rec.UUID+"|"+view)
            }
        }
        g.mu.Unlock()
        return err
    }

    g.mu.Lock()
    for _, rec := range pending {
        for _, view := range rec.Views {
            g.applied[rec.UUID+"|"+view] = rec
        }
    }
    g.mu.Unlock()

    return nil
}

// changed reports whether rec differs from the last applied record for any of its views; caller must hold g.mu.
func (g *dnsGate) changed(rec update.Record) bool {
    for _, view := range rec.Views {
        prev, ok := g.applied[rec.UUID+"|"+view]
        if !ok || !recordsEqual(prev, rec) {
            return true
        }
    }
    return false
}

func (g *dnsGate) Delete(id string, views ...string) error {
    if err := g.delete(id, views...); err != nil {
        return err
    }

    g.mu.Lock()
    defer g.mu.Unlock()
    prefix := id + "|"
    if len(views) == 0 {
        for key := range g.applied {
            if strings.HasPrefix(key, prefix) {
                delete(g.applied, key)
            }
        }
        return nil
    }
    for _, view := range views {
        delete(g.applied, prefix+view)
    }
    return nil
}

func recordsEqual(a, b update.Record) bool {
    if a.Name != b.Name || a.UUID != b.UUID {
        return false
    }
    if a.Address == nil || b.Address == nil {
        return a.Address == b.Address
    }
    return a.Address.String() == b.Address.String()
}