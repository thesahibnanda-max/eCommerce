package dependencyInjection

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	instamojoClient "github.com/thesahibnanda-max/eCommerce/backend/package/clients/instamojo"
	selfUPIClient "github.com/thesahibnanda-max/eCommerce/backend/package/clients/self_upi"
	paymentGateway "github.com/thesahibnanda-max/eCommerce/backend/package/payment_gateway"
	rateLimiter "github.com/thesahibnanda-max/eCommerce/backend/package/rate_limiter"
	addressRepository "github.com/thesahibnanda-max/eCommerce/backend/package/repository/address"
	cartsRepository "github.com/thesahibnanda-max/eCommerce/backend/package/repository/carts"
	imagesRepository "github.com/thesahibnanda-max/eCommerce/backend/package/repository/images"
	ordersRepository "github.com/thesahibnanda-max/eCommerce/backend/package/repository/orders"
	paymentsRepository "github.com/thesahibnanda-max/eCommerce/backend/package/repository/payments"
	productsRepository "github.com/thesahibnanda-max/eCommerce/backend/package/repository/products"
	spreadsheetRepository "github.com/thesahibnanda-max/eCommerce/backend/package/repository/spreadsheet"
	usersRepository "github.com/thesahibnanda-max/eCommerce/backend/package/repository/users"
	cronService "github.com/thesahibnanda-max/eCommerce/backend/package/services/cron"
	snapshotService "github.com/thesahibnanda-max/eCommerce/backend/package/services/snapshot"
	"github.com/thesahibnanda-max/eCommerce/backend/package/smtp"
	staticInfo "github.com/thesahibnanda-max/eCommerce/backend/package/static_info"
)

func joinClose(closeFns []func() error) func() error {
	return func() error {
		var errs []error
		for i := len(closeFns) - 1; i >= 0; i-- {
			if err := closeFns[i](); err != nil {
				errs = append(errs, err)
			}
		}
		if len(errs) == 0 {
			return nil
		}
		return fmt.Errorf("%w: %w", ErrCloseFailed, errors.Join(errs...))
	}
}

func closeOnError(err error, closeFns []func() error) error {
	if closeErr := joinClose(closeFns)(); closeErr != nil {
		return errors.Join(err, closeErr)
	}
	return err
}

func validatePort(port uint) error {
	if port == 0 || port > maxPort {
		return ErrServerPortInvalid
	}
	return nil
}

func prepareStorage(directory string, permission string) (string, os.FileMode, error) {
	directory = strings.TrimSpace(directory)
	if directory == "" {
		return "", 0, ErrStorageDirectoryRequired
	}
	mode, err := strconv.ParseUint(strings.TrimSpace(permission), 8, 32)
	if err != nil || mode == 0 || mode > uint64(os.ModePerm) {
		return "", 0, ErrStoragePermissionInvalid
	}
	directory = filepath.Clean(directory)
	if err := os.MkdirAll(directory, storageDirectoryMode); err != nil {
		return "", 0, fmt.Errorf("%w: %w", ErrStorageDirectoryFailed, err)
	}
	return directory, os.FileMode(mode), nil
}

func storagePath(directory string, file string) []string {
	return []string{directory, strings.TrimSpace(file)}
}

func smtpSenders(configs []senderConfig) ([]smtp.SenderConfig, error) {
	senders := make([]smtp.SenderConfig, 0, len(configs))
	for _, cfg := range configs {
		security, err := smtpSecurity(cfg.Security)
		if err != nil {
			return nil, err
		}
		senders = append(senders, smtp.SenderConfig{
			Email:    cfg.Email,
			Host:     cfg.Host,
			Port:     cfg.Port,
			Username: cfg.Username,
			Password: cfg.Password,
			Security: security,
			Timeout:  cfg.Timeout,
		})
	}
	return senders, nil
}

func smtpSecurity(value string) (smtp.Security, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case securityStartTLS:
		return smtp.SecurityStartTLS, nil
	case securitySSL:
		return smtp.SecuritySSL, nil
	case securityNone:
		return smtp.SecurityNone, nil
	default:
		return 0, ErrSMTPSecurityInvalid
	}
}

func limiterConfig(cfg rateLimitConfig) rateLimiter.Config {
	rules := make(map[string]rateLimiter.Rule, len(cfg.Rules))
	for name, rule := range cfg.Rules {
		rules[name] = rateLimiter.Rule{RequestsPerMinute: rule.RequestsPerMinute, Burst: rule.Burst}
	}
	return rateLimiter.Config{CacheSize: cfg.CacheSize, TTL: cfg.TTL, Rules: rules}
}

func staticInfoConfig(config configStruct) staticInfo.Config {
	return staticInfo.Config{
		CompanyName:       config.App.CompanyName,
		SupportEmail:      config.App.SupportEmail,
		SupportPhone:      config.App.SupportPhone,
		Address:           config.App.Address,
		GrievanceOfficer:  config.App.GrievanceOfficer,
		EffectiveDate:     config.Policies.EffectiveDate,
		ReturnWindowDays:  config.Policies.ReturnWindowDays,
		RefundDays:        config.Policies.RefundDays,
		DispatchDays:      config.Policies.DispatchDays,
		DeliveryDays:      config.Policies.DeliveryDays,
		FreeShippingAbove: config.Policies.FreeShippingAbove,
		ShippingFee:       config.Policies.ShippingFee,
	}
}

type paymentRelay struct {
	mu     sync.RWMutex
	handle paymentGateway.FinalizedFunc
}

func (r *paymentRelay) set(handle paymentGateway.FinalizedFunc) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.handle = handle
}

func (r *paymentRelay) forward(ctx context.Context, payment paymentGateway.Payment) {
	r.mu.RLock()
	handle := r.handle
	r.mu.RUnlock()
	if handle != nil {
		handle(ctx, payment)
	}
}

func newPaymentGateway(cfg paymentsConfig, companyName, directory string, permission os.FileMode, payments paymentsRepository.Repository, relay *paymentRelay) (paymentGateway.Gateway, []func() error, error) {
	logger := slog.Default()
	switch strings.ToUpper(strings.TrimSpace(cfg.Provider)) {
	case paymentGateway.ProviderSelfUPI.String():
		client, err := selfUPIClient.NewClient()
		if err != nil {
			return nil, nil, err
		}
		payee := cfg.UPI.PayeeName
		if strings.TrimSpace(payee) == "" {
			payee = companyName
		}
		gateway, err := paymentGateway.NewGateway(paymentGateway.UPIConstructorParams{
			Client:      client,
			Payments:    payments,
			PayeeName:   payee,
			Logger:      logger,
			OnFinalized: relay.forward,
		})
		if err != nil {
			return nil, nil, err
		}
		return gateway, []func() error{gateway.Close}, nil
	case paymentGateway.ProviderInstamojo.String():
		client, err := instamojoClient.NewClient(cfg.Instamojo.BaseURL, cfg.Instamojo.ClientID, cfg.Instamojo.ClientSecret, cfg.Instamojo.Timeout)
		if err != nil {
			return nil, nil, err
		}
		gateway, err := paymentGateway.NewGateway(paymentGateway.InstamojoConstructorParams{
			Client:          client,
			Payments:        payments,
			PrivateSalt:     cfg.Instamojo.PrivateSalt,
			QueuePath:       storagePath(directory, cfg.QueueFile),
			QueuePermission: permission,
			PollInterval:    cfg.Instamojo.PollInterval,
			PollWindow:      cfg.Instamojo.PollWindow,
			LeaseTimeout:    cfg.Instamojo.LeaseTimeout,
			Concurrency:     cfg.Instamojo.Concurrency,
			RedirectURL:     cfg.Instamojo.RedirectURL,
			WebhookURL:      cfg.Instamojo.WebhookURL,
			Logger:          logger,
			OnFinalized:     relay.forward,
		})
		if err != nil {
			return nil, []func() error{client.Close}, err
		}
		return gateway, []func() error{client.Close, gateway.Close}, nil
	default:
		return nil, nil, ErrPaymentProviderInvalid
	}
}

func newSnapshots(ctx context.Context, cfg configStruct, sources []snapshotService.Source) (snapshotService.Service, []func() error, error) {
	google := cfg.Google
	if strings.TrimSpace(google.ClientID) == "" || strings.TrimSpace(google.ClientSecret) == "" || strings.TrimSpace(google.RefreshToken) == "" {
		return nil, nil, nil
	}
	var closers []func() error
	sheets, err := spreadsheetRepository.NewRepository(ctx, spreadsheetRepository.Config{
		ClientID:     google.ClientID,
		ClientSecret: google.ClientSecret,
		RefreshToken: google.RefreshToken,
		FolderID:     google.FolderID,
		CompanyName:  cfg.App.CompanyName,
		Timeout:      google.Timeout,
	})
	if err != nil {
		return nil, closers, err
	}
	closers = append(closers, sheets.Close)
	logger := slog.Default()
	snapshots, err := snapshotService.NewService(snapshotService.Params{Spreadsheets: sheets, Sources: sources, Logger: logger})
	if err != nil {
		return nil, closers, err
	}
	engine, err := cronService.ParseEngine(cfg.Cron.Engine)
	if err != nil {
		return nil, closers, err
	}
	params, err := cronService.Params(engine, []cronService.Job{{
		Name:     snapshotJobName,
		Interval: cfg.Snapshot.Interval,
		Timeout:  cfg.Snapshot.Timeout,
		Run: func(ctx context.Context) error {
			_, err := snapshots.Run(ctx)
			if errors.Is(err, snapshotService.ErrAlreadyRunning) {
				return nil
			}
			return err
		},
	}}, logger)
	if err != nil {
		return nil, closers, err
	}
	scheduler, err := cronService.NewService(params)
	if err != nil {
		return nil, closers, err
	}
	if err := scheduler.Start(); err != nil {
		return nil, closers, err
	}
	closers = append(closers, scheduler.Stop)
	return snapshots, closers, nil
}

func snapshotSources(users usersRepository.Repository, products productsRepository.Repository, images imagesRepository.Repository, carts cartsRepository.Repository, addresses addressRepository.Repository, orders ordersRepository.Repository, payments paymentsRepository.Repository) []snapshotService.Source {
	return []snapshotService.Source{
		{Database: "users", Dump: users.Dump},
		{Database: "products", Dump: products.Dump},
		{Database: "images", Dump: images.Dump},
		{Database: "carts", Dump: carts.Dump},
		{Database: "addresses", Dump: addresses.Dump},
		{Database: "orders", Dump: orders.Dump},
		{Database: "payments", Dump: payments.Dump},
	}
}
