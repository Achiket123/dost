package repository

const (
	// TypeUnspecified means not specified, should not be used.
	TypeUnspecified = "0"
	// TypeString means openAPI string type
	TypeString = "1"
	// TypeNumber means openAPI number type
	TypeNumber = "2"
	// TypeInteger means openAPI integer type
	TypeInteger = "3"
	// TypeBoolean means openAPI boolean type
	TypeBoolean = "4"
	// TypeArray means openAPI array type
	TypeArray = "5"
	// TypeObject means openAPI object type
	TypeObject = "6"
)

type Response struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text         string `json:"text"`
				FunctionCall *struct {
					Name string `json:"name"`
					Args map[string]any
				} `json:"functionCall"`
			} `json:"parts"`
			Role string `json:"role"`
		} `json:"content"`
	} `json:"candidates"`
}
type ToolFunction interface {
	Run(args map[string]any) string
	ToObject() map[string]any
}
type Return map[string]any
type Properties struct {
	Type        string                 `json:"type"`
	Items       *Properties            `json:"items"`
	Format      string                 `json:"format"`
	Enum        []string               `json:"enum"`
	Properties  map[string]*Properties `json:"properties"`
	Required    []string               `json:"required"`
	Description string                 `json:"description"`
}

func (p *Properties) ToObject() map[string]any {
	return map[string]any{
		"type":        p.Type,
		"items":       p.Items,
		"format":      p.Format,
		"description": p.Description,
		"enum":        p.Enum,
	}
}

type Parameters struct {
	Type       string                 `json:"type"`
	Properties map[string]*Properties `json:"properties"`
	Required   []string               `json:"required"`
	Optional   []string               `json:"optional"`
}

func (p *Parameters) ToObject() map[string]any {
	temp := make(map[string]any)
	for k, v := range p.Properties {
		temp[k] = v.ToObject()
	}
	val := map[string]any{
		"type":       p.Type,
		"properties": temp,
		"required":   p.Required,
	}
	return val
}

type Function struct {
	Name        string                                   `json:"name"`
	Description string                                   `json:"description"`
	Parameters  Parameters                               `json:"parameters"`
	Return      Return                                   `json:"return"`
	Service     func(args map[string]any) map[string]any `json:"-"`
}

func (f *Function) Run(args map[string]any) map[string]any {
	err := f.Service(args)

	return err
}

func (f *Function) ToObject() map[string]any {

	val := map[string]any{
		"name":        f.Name,
		"description": f.Description,
		"parameters":  f.Parameters.ToObject(),
	}
	return val
}
