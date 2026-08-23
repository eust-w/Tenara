// Package plan renders the CREATE checklist presented before approval
// (plan tenara-agent-paas#18, RB-11 R9 R10).
package plan

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"tenara/control-plane/internal/appspec"
)

// ServiceResources are the per-type estimates shown for approval; they must
// stay within the RB-29 free-tier ceilings enforced again at quota time.
type ServiceResources struct {
	CPUMillicores   int `json:"cpu_millicores"`
	MemoryMegabytes int `json:"memory_megabytes"`
}

type PlanService struct {
	Name      string           `json:"name"`
	Resources ServiceResources `json:"resources"`
}

type Plan struct { //nolint:fieldalignment // JSON wire contract order; layout irrelevant
	Services      []PlanService `json:"services"`
	ExpiresAt     time.Time     `json:"expires_at"`
	NamespaceName string        `json:"namespace_name"`
	DatabaseName  string        `json:"database_name,omitempty"`
	Domain        string        `json:"domain"`
	PlanID        string        `json:"plan_id,omitempty"`
}

var resourceByType = map[string]ServiceResources{
	"frontend": {CPUMillicores: 250, MemoryMegabytes: 256},
	"backend":  {CPUMillicores: 500, MemoryMegabytes: 512},
}

type Input struct {
	Spec       appspec.Spec `json:"-"`
	Now        time.Time
	AppID      string // full uuid; first segment labels ns/db/domain
	Slug       string // application slug
	Env        string // production|staging|preview-*
	BaseDomain string // e.g. tenara.local / 127.0.0.1.nip.io
	TTL        time.Duration
}

// Generate renders a deterministic checklist; nothing here touches clusters.
func Generate(in Input) (Plan, error) {
	if in.AppID == "" || in.Slug == "" || in.Env == "" || in.BaseDomain == "" {
		return Plan{}, errors.New("plan input incomplete")
	}
	shortID := strings.SplitN(in.AppID, "-", 2)[0]

	names := make([]string, 0, len(in.Spec.Services))
	for name := range in.Spec.Services {
		names = append(names, name)
	}
	sort.Strings(names)

	services := make([]PlanService, 0, len(names))
	for _, name := range names {
		def := in.Spec.Services[name]
		res, ok := resourceByType[def.Type]
		if !ok {
			return Plan{}, fmt.Errorf("no resource estimate for type %q", def.Type)
		}
		services = append(services, PlanService{Name: name, Resources: res})
	}

	databaseName := ""
	if in.Spec.Database != nil && in.Spec.Database.MongoDB {
		databaseName = "app_" + shortID
	}

	domain := fmt.Sprintf("%s.%s", in.Slug, in.BaseDomain)
	if in.Env != "production" {
		domain = fmt.Sprintf("%s-%s.%s", in.Slug, in.Env, in.BaseDomain)
	}

	return Plan{
		NamespaceName: fmt.Sprintf("app-%s-%s", shortID, in.Env),
		Services:      services,
		DatabaseName:  databaseName,
		Domain:        domain,
		ExpiresAt:     in.Now.Add(in.TTL),
	}, nil
}
