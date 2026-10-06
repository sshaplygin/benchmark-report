package model

type Platform struct {
	OS   string `json:"os"`
	Arch string `json:"arch"`
}
type File struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Role   string `json:"role"`
}
type Replay struct {
	Render []string `json:"render"`
	Verify []string `json:"verify,omitempty"`
}
type ReportInvocation struct {
	Parser        string `json:"parser"`
	CommentHeader string `json:"comment_header"`
	ArtifactURL   string `json:"artifact_url"`
}
type Reproduction struct {
	Report              *ReportInvocation `json:"report,omitempty"`
	SchemaVersion       int               `json:"schema_version"`
	Generator           Tool              `json:"generator"`
	Platform            Platform          `json:"platform"`
	Base                Side              `json:"base"`
	Head                Side              `json:"head"`
	ConfigurationSHA256 string            `json:"configuration_sha256"`
	Files               []File            `json:"files"`
	Statistics          []Statistics      `json:"statistics"`
	Replay              Replay            `json:"replay"`
}
