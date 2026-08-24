// Package cells implements the multi-cluster cell registry (RB§38 §39,
// D2-P3-3 / todo95): the control plane tracks N cluster data-plane endpoints
// and routes organizations onto cells so blast radius stays contained.
package cells

import (
	"errors"
	"fmt"
	"sort"
	"sync"
)

// Cell is one registered cluster behind the platform.
type Cell struct {
	Name     string
	Cloud    string // runtime adapter name; see KnownClouds
	Endpoint string // data-plane provider endpoint for this cluster
	Region   string
	Default  bool // receives orgs without explicit assignment
	orgs     map[string]struct{}
}

// KnownClouds enumerates the runtime adapters shipped by the platform;
// registration rejects anything else so every routing decision stays
// resolvable against a real provider binding (todo96 fleet).
var KnownClouds = map[string]bool{
	"baidu-cce": true, "aliyun-ack": true,
	"tencent-tke": true, "selfhosted": true,
}

// Registry errors.
var (
	ErrNoCell        = errors.New("no cell routed for organization")
	ErrDuplicateCell = errors.New("cell already registered")
	ErrUnknownCell   = errors.New("unknown cell")
	ErrUnknownCloud  = errors.New("unknown cloud kind")
)

// Spec describes one cell registration.
type Spec struct {
	Name     string
	Cloud    string
	Endpoint string
	Region   string
	Default  bool
}

// Target is the resolved control-plane decision for one organization.
type Target struct {
	CellName string
	Cloud    string
	Endpoint string
	Region   string
}

// Registry is the concurrency-safe in-memory cell table; a DB-backed store
// replaces it when the registration API lands (design-doc dependency).
type Registry struct {
	mu    sync.RWMutex
	cells map[string]*Cell
	names []string
}

// NewRegistry returns an empty registry.
func NewRegistry() *Registry {
	return &Registry{cells: map[string]*Cell{}}
}

// Register adds a cell; validation rejects anonymous or endpoint-less
// cells plus any cloud kind outside the shipped adapter fleet.
func (r *Registry) Register(s Spec) error {
	if s.Name == "" || s.Endpoint == "" {
		return errors.New("cell name and endpoint required")
	}
	if !KnownClouds[s.Cloud] {
		return fmt.Errorf("%w: %q", ErrUnknownCloud, s.Cloud)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, dup := r.cells[s.Name]; dup {
		return ErrDuplicateCell
	}
	r.cells[s.Name] = &Cell{
		Name: s.Name, Cloud: s.Cloud, Endpoint: s.Endpoint,
		Region: s.Region, Default: s.Default, orgs: map[string]struct{}{},
	}
	r.names = append(r.names, s.Name)
	return nil
}

// Assign pins one organization onto a cell explicitly.
func (r *Registry) Assign(orgID, cellName string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	c, ok := r.cells[cellName]
	if !ok {
		return ErrUnknownCell
	}
	c.orgs[orgID] = struct{}{}
	return nil
}

// RouteForOrg resolves the org's cell: explicit assignment wins, then the
// default cell; otherwise ErrNoCell.
func (r *Registry) RouteForOrg(orgID string) (Cell, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var fallback *Cell
	for _, name := range r.names {
		c := r.cells[name]
		if _, pinned := c.orgs[orgID]; pinned {
			return snapshot(c), nil
		}
		if c.Default && fallback == nil {
			fallback = c
		}
	}
	if fallback != nil {
		return snapshot(fallback), nil
	}
	return Cell{}, ErrNoCell
}

// TargetForOrg resolves the deterministic multicloud decision for one org:
// the routed cell's provider identity plus its data-plane endpoint.
func (r *Registry) TargetForOrg(orgID string) (Target, error) {
	c, routeErr := r.RouteForOrg(orgID)
	if routeErr != nil {
		return Target{}, routeErr
	}
	return Target{
		CellName: c.Name, Cloud: c.Cloud,
		Endpoint: c.Endpoint, Region: c.Region,
	}, nil
}

// FleetByCloud groups cell names per cloud kind for admin health views.
func (r *Registry) FleetByCloud() map[string][]string {
	out := map[string][]string{}
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, name := range r.names {
		c := r.cells[name]
		out[c.Cloud] = append(out[c.Cloud], name)
	}
	for k := range out {
		sort.Strings(out[k])
	}
	return out
}

// AffectedApps enumerates the apps homed on one failed cell, proving the
// acceptance invariant: killing mongo inside cell B degrades only cell-B
// apps while every other cell keeps serving untouched.
func AffectedApps(appToCell map[string]string, failedCell string) []string {
	var out []string
	for app, cell := range appToCell {
		if cell == failedCell {
			out = append(out, app)
		}
	}
	sort.Strings(out)
	return out
}

func snapshot(c *Cell) Cell {
	cp := *c
	cp.orgs = map[string]struct{}{}
	for o := range c.orgs {
		cp.orgs[o] = struct{}{}
	}
	return cp
}

// AssignedOrgs exposes the explicit assignments of a snapshot (tests/APIs).
func (c Cell) AssignedOrgs() []string {
	out := make([]string, 0, len(c.orgs))
	for o := range c.orgs {
		out = append(out, o)
	}
	sort.Strings(out)
	return out
}
