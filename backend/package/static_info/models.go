package staticInfo

type Config struct {
	CompanyName       string
	SupportEmail      string
	SupportPhone      string
	Address           string
	GrievanceOfficer  string
	EffectiveDate     string
	ReturnWindowDays  uint
	RefundDays        string
	DispatchDays      string
	DeliveryDays      string
	FreeShippingAbove uint
	ShippingFee       uint
}

type Contact struct {
	CompanyName      string `json:"company_name"`
	Email            string `json:"email"`
	Phone            string `json:"phone"`
	Address          string `json:"address"`
	GrievanceOfficer string `json:"grievance_officer"`
}

type Section struct {
	Heading    string   `json:"heading" yaml:"heading"`
	Paragraphs []string `json:"paragraphs,omitempty" yaml:"paragraphs"`
	Bullets    []string `json:"bullets,omitempty" yaml:"bullets"`
}

type Page struct {
	Slug          string    `json:"slug" yaml:"slug"`
	Title         string    `json:"title" yaml:"title"`
	Summary       string    `json:"summary" yaml:"summary"`
	EffectiveDate string    `json:"effective_date" yaml:"-"`
	Sections      []Section `json:"sections" yaml:"sections"`
}

type PageSummary struct {
	Slug    string `json:"slug"`
	Title   string `json:"title"`
	Summary string `json:"summary"`
}

type document struct {
	Pages []Page `yaml:"pages"`
}

type templateData struct {
	CompanyName       string
	SupportEmail      string
	SupportPhone      string
	Address           string
	GrievanceOfficer  string
	EffectiveDate     string
	ReturnWindowDays  uint
	RefundDays        string
	DispatchDays      string
	DeliveryDays      string
	FreeShippingAbove string
	ShippingFee       string
}
