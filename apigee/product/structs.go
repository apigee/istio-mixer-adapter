package product

type apiResponse struct {
	APIProducts []Details `json:"apiProducts"`
}

type Details struct {
	Attributes     []Attribute `json:"attributes,omitempty"`
	CreatedAt      string      `json:"createdAt,omitempty"`
	CreatedBy      string      `json:"createdBy,omitempty"`
	Description    string      `json:"description,omitempty"`
	DisplayName    string      `json:"displayName,omitempty"`
	Environments   []string    `json:"environments,omitempty"`
	LastModifiedAt string      `json:"lastModifiedAt,omitempty"`
	LastModifiedBy string      `json:"lastModifiedBy,omitempty"`
	Name           string      `json:"name,omitempty"`
	QuotaLimit     string      `json:"quota,omitempty"`
	QuotaInterval  int64       `json:"quotaInterval,omitempty"`
	QuotaTimeUnit  string      `json:"quotaTimeUnit,omitempty"`
	Resources      []string    `json:"apiResources"`
	Scopes         []string    `json:"scopes"`
}

type Attribute struct {
	Kind  string `json:"kind,omitempty"`
	Name  string `json:"name,omitempty"`
	Value string `json:"value,omitempty"`
}
