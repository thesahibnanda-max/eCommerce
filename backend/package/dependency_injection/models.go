package dependencyInjection

import (
	"time"
)

type configStruct struct {
	App struct {
		CompanyName      string `yaml:"companyName"`
		SupportEmail     string `yaml:"supportEmail"`
		SupportPhone     string `yaml:"supportPhone"`
		Address          string `yaml:"address"`
		GrievanceOfficer string `yaml:"grievanceOfficer"`
		StoreURL         string `yaml:"storeURL"`
	} `yaml:"app"`
	Policies struct {
		EffectiveDate     string `yaml:"effectiveDate"`
		ReturnWindowDays  uint   `yaml:"returnWindowDays"`
		RefundDays        string `yaml:"refundDays"`
		DispatchDays      string `yaml:"dispatchDays"`
		DeliveryDays      string `yaml:"deliveryDays"`
		FreeShippingAbove uint   `yaml:"freeShippingAbove"`
		ShippingFee       uint   `yaml:"shippingFee"`
	} `yaml:"policies"`
	Server struct {
		Port uint `yaml:"port"`
	} `yaml:"server"`
	CORS struct {
		FrontendPort uint `yaml:"frontendPort"`
	} `yaml:"cors"`
	Token struct {
		Secret string `yaml:"secret"`
	} `yaml:"token"`
	Admin struct {
		Key string `yaml:"key"`
	} `yaml:"admin"`
	Storage struct {
		Directory     string `yaml:"directory"`
		Permission    string `yaml:"permission"`
		UsersFile     string `yaml:"usersFile"`
		ProductsFile  string `yaml:"productsFile"`
		ImagesFile    string `yaml:"imagesFile"`
		CartsFile     string `yaml:"cartsFile"`
		OrdersFile    string `yaml:"ordersFile"`
		AddressesFile string `yaml:"addressesFile"`
	} `yaml:"storage"`
	Users struct {
		IsOneValidationEnough bool `yaml:"isOneValidationEnough"`
	} `yaml:"users"`
	SMTP struct {
		Senders []senderConfig `yaml:"senders"`
	} `yaml:"smtp"`
	Otp struct {
		Email struct {
			Length         uint          `yaml:"length"`
			TTL            time.Duration `yaml:"ttl"`
			ResendCooldown time.Duration `yaml:"resendCooldown"`
			MaxAttempts    int           `yaml:"maxAttempts"`
			Capacity       uint64        `yaml:"capacity"`
			Subject        string        `yaml:"subject"`
		} `yaml:"email"`
	} `yaml:"otp"`
	RateLimit rateLimitConfig `yaml:"rateLimit"`
	Payments  paymentsConfig  `yaml:"payments"`
}

type paymentsConfig struct {
	Provider     string                  `yaml:"provider"`
	PaymentsFile string                  `yaml:"paymentsFile"`
	QueueFile    string                  `yaml:"queueFile"`
	Instamojo    instamojoPaymentsConfig `yaml:"instamojo"`
	UPI          upiPaymentsConfig       `yaml:"upi"`
}

type instamojoPaymentsConfig struct {
	BaseURL      string        `yaml:"baseURL"`
	ClientID     string        `yaml:"clientID"`
	ClientSecret string        `yaml:"clientSecret"`
	PrivateSalt  string        `yaml:"privateSalt"`
	Timeout      time.Duration `yaml:"timeout"`
	PollInterval time.Duration `yaml:"pollInterval"`
	PollWindow   time.Duration `yaml:"pollWindow"`
	LeaseTimeout time.Duration `yaml:"leaseTimeout"`
	Concurrency  int           `yaml:"concurrency"`
	RedirectURL  string        `yaml:"redirectURL"`
	WebhookURL   string        `yaml:"webhookURL"`
}

type upiPaymentsConfig struct {
	PayeeName string `yaml:"payeeName"`
}

type senderConfig struct {
	Email    string        `yaml:"email"`
	Host     string        `yaml:"host"`
	Port     int           `yaml:"port"`
	Username string        `yaml:"username"`
	Password string        `yaml:"password"`
	Security string        `yaml:"security"`
	Timeout  time.Duration `yaml:"timeout"`
}

type rateLimitConfig struct {
	CacheSize int                      `yaml:"cacheSize"`
	TTL       time.Duration            `yaml:"ttl"`
	Rules     map[string]rateLimitRule `yaml:"rules"`
}

type rateLimitRule struct {
	RequestsPerMinute float64 `yaml:"requestsPerMinute"`
	Burst             int     `yaml:"burst"`
}
