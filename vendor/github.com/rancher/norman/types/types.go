package types

const (
	ResourceFieldID = "id"
)

type Collection struct {
	Type         string                 `json:"type,omitempty"`
	Links        map[string]string      `json:"links"`
	CreateTypes  map[string]string      `json:"createTypes,omitempty"`
	Actions      map[string]string      `json:"actions"`
	Pagination   *Pagination            `json:"pagination,omitempty"`
	Sort         *Sort                  `json:"sort,omitempty"`
	Filters      map[string][]Condition `json:"filters,omitempty"`
	ResourceType string                 `json:"resourceType"`
}

type GenericCollection struct {
	Collection
	Data []interface{} `json:"data"`
}

type ResourceCollection struct {
	Collection
	Data []Resource `json:"data,omitempty"`
}

type SortOrder string

type Sort struct {
	Name    string            `json:"name,omitempty"`
	Order   SortOrder         `json:"order,omitempty"`
	Reverse string            `json:"reverse,omitempty"`
	Links   map[string]string `json:"links,omitempty"`
}

var (
	ModifierEQ      ModifierType = "eq"
	ModifierNE      ModifierType = "ne"
	ModifierNull    ModifierType = "null"
	ModifierNotNull ModifierType = "notnull"
	ModifierIn      ModifierType = "in"
	ModifierNotIn   ModifierType = "notin"
)

type ModifierType string

type Condition struct {
	Modifier ModifierType `json:"modifier,omitempty"`
	Value    interface{}  `json:"value,omitempty"`
}

type Pagination struct {
	Marker   string `json:"marker,omitempty"`
	First    string `json:"first,omitempty"`
	Previous string `json:"previous,omitempty"`
	Next     string `json:"next,omitempty"`
	Last     string `json:"last,omitempty"`
	Limit    *int64 `json:"limit,omitempty"`
	Total    *int64 `json:"total,omitempty"`
	Partial  bool   `json:"partial,omitempty"`
}

type Resource struct {
	ID      string            `json:"id,omitempty"`
	Type    string            `json:"type,omitempty"`
	Links   map[string]string `json:"links"`
	Actions map[string]string `json:"actions"`
}

type NamedResource struct {
	Resource
	Name        string `json:"name"`
	Description string `json:"description"`
}

type NamedResourceCollection struct {
	Collection
	Data []NamedResource `json:"data,omitempty"`
}

type APIVersion struct {
	Group            string `json:"group,omitempty"`
	Version          string `json:"version,omitempty"`
	Path             string `json:"path,omitempty"`
	SubContext       bool   `json:"subContext,omitempty"`
	SubContextSchema string `json:"filterField,omitempty"`
}

type Namespaced struct{}

var NamespaceScope TypeScope = "namespace"

type TypeScope string

type Schema struct {
	ID                string            `json:"id,omitempty"`
	Embed             bool              `json:"embed,omitempty"`
	EmbedType         string            `json:"embedType,omitempty"`
	CodeName          string            `json:"-"`
	CodeNamePlural    string            `json:"-"`
	PkgName           string            `json:"-"`
	Type              string            `json:"type,omitempty"`
	BaseType          string            `json:"baseType,omitempty"`
	Links             map[string]string `json:"links"`
	Version           APIVersion        `json:"version"`
	PluralName        string            `json:"pluralName,omitempty"`
	ResourceMethods   []string          `json:"resourceMethods,omitempty"`
	ResourceFields    map[string]Field  `json:"resourceFields"`
	ResourceActions   map[string]Action `json:"resourceActions,omitempty"`
	CollectionMethods []string          `json:"collectionMethods,omitempty"`
	CollectionFields  map[string]Field  `json:"collectionFields,omitempty"`
	CollectionActions map[string]Action `json:"collectionActions,omitempty"`
	CollectionFilters map[string]Filter `json:"collectionFilters,omitempty"`
	Scope             TypeScope         `json:"-"`

	InternalSchema      *Schema             `json:"-"`
	Mapper              Mapper              `json:"-"`
	ActionHandler       ActionHandler       `json:"-"`
	LinkHandler         RequestHandler      `json:"-"`
	ListHandler         RequestHandler      `json:"-"`
	CreateHandler       RequestHandler      `json:"-"`
	DeleteHandler       RequestHandler      `json:"-"`
	UpdateHandler       RequestHandler      `json:"-"`
	InputFormatter      InputFormatter      `json:"-"`
	Formatter           Formatter           `json:"-"`
	CollectionFormatter CollectionFormatter `json:"-"`
	ErrorHandler        ErrorHandler        `json:"-"`
	Validator           Validator           `json:"-"`
	Store               Store               `json:"-"`
}

type Field struct {
	Type         string      `json:"type,omitempty"`
	Default      interface{} `json:"default,omitempty"`
	Nullable     bool        `json:"nullable,omitempty"`
	Create       bool        `json:"create"`
	WriteOnly    bool        `json:"writeOnly,omitempty"`
	Required     bool        `json:"required,omitempty"`
	Update       bool        `json:"update"`
	MinLength    *int64      `json:"minLength,omitempty"`
	MaxLength    *int64      `json:"maxLength,omitempty"`
	Min          *int64      `json:"min,omitempty"`
	Max          *int64      `json:"max,omitempty"`
	Options      []string    `json:"options,omitempty"`
	ValidChars   string      `json:"validChars,omitempty"`
	InvalidChars string      `json:"invalidChars,omitempty"`
	Description  string      `json:"description,omitempty"`
	CodeName     string      `json:"-"`
	DynamicField bool        `json:"dynamicField,omitempty"`
}

type Action struct {
	Input  string `json:"input,omitempty"`
	Output string `json:"output,omitempty"`
}

type Filter struct {
	Modifiers []ModifierType `json:"modifiers,omitempty"`
}

type ListOpts struct {
	Filters map[string]interface{}
}

func (c *Collection) AddAction(apiContext *APIContext, name string) {
	c.Actions[name] = apiContext.URLBuilder.CollectionAction(apiContext.Schema, nil, name)
}
