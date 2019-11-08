package api

// ProjectsPost represents the fields of a new LXD project
//
// API extension: projects
type ProjectsPost struct {
	ProjectPut `yaml:",inline"`

	Name string `json:"name" yaml:"name"`
}

// ProjectPost represents the fields required to rename a LXD project
//
// API extension: projects
type ProjectPost struct {
	Name string `json:"name" yaml:"name"`
}

// ProjectPut represents the modifiable fields of a LXD project
//
// API extension: projects
type ProjectPut struct {
	Description string            `json:"description" yaml:"description"`
	Config      map[string]string `json:"config" yaml:"config"`
}

// Project represents a LXD project
//
// API extension: projects
type Project struct {
	ProjectPut `yaml:",inline"`

	Name   string   `json:"name" yaml:"name"`
	UsedBy []string `json:"used_by" yaml:"used_by"`
}

// Writable converts a full Project struct into a ProjectPut struct (filters read-only fields)
//
// API extension: projects
func (project *Project) Writable() ProjectPut {
	return project.ProjectPut
}
