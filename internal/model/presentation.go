package model

type PresentationSections struct {
	Metadata  bool `json:"metadata"`
	Summary   bool `json:"summary"`
	Tables    bool `json:"tables"`
	Benchstat bool `json:"benchstat"`
}
type PresentationSummary struct {
	Total          int `json:"total"`
	Selected       int `json:"selected"`
	Displayed      int `json:"displayed"`
	Omitted        int `json:"omitted"`
	Regression     int `json:"regression"`
	Improvement    int `json:"improvement"`
	BelowThreshold int `json:"below_threshold"`
	NotComparable  int `json:"not_comparable"`
}
type PresentationLabel struct {
	Field string `json:"field"`
	Value string `json:"value"`
}
type PresentationRow struct {
	Key      string   `json:"key"`
	Identity Identity `json:"identity"`
	Base     string   `json:"base"`
	Head     string   `json:"head"`
	Change   string   `json:"change"`
	Signal   string   `json:"signal"`
	Samples  string   `json:"samples"`
}
type PresentationGroup struct {
	Labels []PresentationLabel `json:"labels"`
	Rows   []PresentationRow   `json:"rows"`
}
type Presentation struct {
	SchemaVersion int                  `json:"schema_version"`
	Title         string               `json:"title"`
	Columns       []string             `json:"columns"`
	Base          Side                 `json:"base"`
	Head          Side                 `json:"head"`
	Sections      PresentationSections `json:"sections"`
	Summary       PresentationSummary  `json:"summary"`
	Disclosures   []string             `json:"disclosures"`
	Groups        []PresentationGroup  `json:"groups"`
}
