package cricinfo

import "encoding/json"

// Ref is the common Cricinfo hypermedia reference object.
type Ref struct {
	URL string `json:"$ref"`
}

// Page is the common paginated response envelope used by many routes.
type Page[T any] struct {
	Count     int `json:"count"`
	Items     []T `json:"items"`
	PageCount int `json:"pageCount"`
	PageIndex int `json:"pageIndex"`
	PageSize  int `json:"pageSize"`
}

// RootDiscovery is a minimal shape for the API root resource.
type RootDiscovery struct {
	Ref     string `json:"$ref,omitempty"`
	ID      string `json:"id,omitempty"`
	Name    string `json:"name,omitempty"`
	Slug    string `json:"slug,omitempty"`
	UID     string `json:"uid,omitempty"`
	Events  *Ref   `json:"events,omitempty"`
	Leagues *Ref   `json:"leagues,omitempty"`
}

// StatsObject is a minimal single-object stats payload shape.
type StatsObject struct {
	Ref         string          `json:"$ref,omitempty"`
	Athlete     *Ref            `json:"athlete,omitempty"`
	Competition *Ref            `json:"competition,omitempty"`
	Team        *Ref            `json:"team,omitempty"`
	Splits      json.RawMessage `json:"splits"`
}

// Document is a fetched JSON payload plus request/canonical metadata.
type Document struct {
	RequestedRef string
	CanonicalRef string
	StatusCode   int
	Body         []byte
}

// ResolvedDocument is the result of following a pointer chain.
type ResolvedDocument struct {
	RequestedRef string
	CanonicalRef string
	TraversedRef []string
	StatusCode   int
	Body         []byte
}
