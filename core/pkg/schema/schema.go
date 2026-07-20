package schema

type FieldType string

const (
	TypeString  FieldType = "string"
	TypeInteger FieldType = "integer"
	TypeBoolean FieldType = "boolean"
)

// Field details a single configuration input rule for the Web UI or validator.
type Field struct {
	Name        string    `json:"name"`
	Type        FieldType `json:"type"`
	Required    bool      `json:"required"`
	IsSecret    bool      `json:"is_secret"`
	Description string    `json:"description"`
}
