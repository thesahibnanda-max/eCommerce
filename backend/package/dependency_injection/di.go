package dependencyInjection

import (
	"context"
	_ "embed"
	"fmt"
	"log/slog"
	"sync"

	"github.com/thesahibnanda-max/eCommerce/backend/package/handler"
	"github.com/thesahibnanda-max/eCommerce/backend/package/otp"
	rateLimiter "github.com/thesahibnanda-max/eCommerce/backend/package/rate_limiter"
	addressRepository "github.com/thesahibnanda-max/eCommerce/backend/package/repository/address"
	cartsRepository "github.com/thesahibnanda-max/eCommerce/backend/package/repository/carts"
	imagesRepository "github.com/thesahibnanda-max/eCommerce/backend/package/repository/images"
	ordersRepository "github.com/thesahibnanda-max/eCommerce/backend/package/repository/orders"
	paymentsRepository "github.com/thesahibnanda-max/eCommerce/backend/package/repository/payments"
	productsRepository "github.com/thesahibnanda-max/eCommerce/backend/package/repository/products"
	usersRepository "github.com/thesahibnanda-max/eCommerce/backend/package/repository/users"
	"github.com/thesahibnanda-max/eCommerce/backend/package/router"
	addressesService "github.com/thesahibnanda-max/eCommerce/backend/package/services/addresses"
	adminService "github.com/thesahibnanda-max/eCommerce/backend/package/services/admin"
	adminOperationsService "github.com/thesahibnanda-max/eCommerce/backend/package/services/admin_operations"
	cartsService "github.com/thesahibnanda-max/eCommerce/backend/package/services/carts"
	imagesService "github.com/thesahibnanda-max/eCommerce/backend/package/services/images"
	ordersService "github.com/thesahibnanda-max/eCommerce/backend/package/services/orders"
	productsService "github.com/thesahibnanda-max/eCommerce/backend/package/services/products"
	staticInfoService "github.com/thesahibnanda-max/eCommerce/backend/package/services/static_info"
	usersService "github.com/thesahibnanda-max/eCommerce/backend/package/services/users"
	"github.com/thesahibnanda-max/eCommerce/backend/package/smtp"
	staticInfo "github.com/thesahibnanda-max/eCommerce/backend/package/static_info"
	"github.com/thesahibnanda-max/eCommerce/backend/package/token"
	"github.com/thesahibnanda-max/eCommerce/backend/package/utils"
)

const (
	maxPort              = 65535
	storageDirectoryMode = 0o700
	securityStartTLS     = "starttls"
	securitySSL          = "ssl"
	securityNone         = "none"
	snapshotJobName      = "snapshot-all"
)

//go:embed config.yaml
var configString string

var loadConfig = sync.OnceValues(func() (configStruct, error) {
	return utils.ReadParametrisedYAML[configStruct](configString)
})

type App interface {
	Close() error
	Router() router.Router
	Port() uint
}

type app struct {
	router    router.Router
	port      uint
	close     func() error
	closeOnce sync.Once
	closeErr  error
}

func NewApp(ctx context.Context) (App, error) {
	if err := utils.CheckContext(ctx); err != nil {
		return nil, err
	}
	config, err := loadConfig()
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrConfigInvalid, err)
	}
	if err := validatePort(config.Server.Port); err != nil {
		return nil, err
	}

	directory, permission, err := prepareStorage(config.Storage.Directory, config.Storage.Permission)
	if err != nil {
		return nil, err
	}

	var closeFns []func() error

	usersRepo, err := usersRepository.NewRepository(storagePath(directory, config.Storage.UsersFile), permission)
	if err != nil {
		return nil, closeOnError(err, closeFns)
	}
	closeFns = append(closeFns, usersRepo.Close)

	productsRepo, err := productsRepository.NewRepository(storagePath(directory, config.Storage.ProductsFile), permission)
	if err != nil {
		return nil, closeOnError(err, closeFns)
	}
	closeFns = append(closeFns, productsRepo.Close)

	imagesRepo, err := imagesRepository.NewRepository(storagePath(directory, config.Storage.ImagesFile), permission)
	if err != nil {
		return nil, closeOnError(err, closeFns)
	}
	closeFns = append(closeFns, imagesRepo.Close)

	cartsRepo, err := cartsRepository.NewRepository(storagePath(directory, config.Storage.CartsFile), permission)
	if err != nil {
		return nil, closeOnError(err, closeFns)
	}
	closeFns = append(closeFns, cartsRepo.Close)

	ordersRepo, err := ordersRepository.NewRepository(storagePath(directory, config.Storage.OrdersFile), permission)
	if err != nil {
		return nil, closeOnError(err, closeFns)
	}
	closeFns = append(closeFns, ordersRepo.Close)

	addressRepo, err := addressRepository.NewRepository(storagePath(directory, config.Storage.AddressesFile), permission)
	if err != nil {
		return nil, closeOnError(err, closeFns)
	}
	closeFns = append(closeFns, addressRepo.Close)

	paymentsRepo, err := paymentsRepository.NewRepository(storagePath(directory, config.Payments.PaymentsFile), permission)
	if err != nil {
		return nil, closeOnError(err, closeFns)
	}
	closeFns = append(closeFns, paymentsRepo.Close)

	tokenizer, err := token.NewTokenizer(config.Token.Secret)
	if err != nil {
		return nil, closeOnError(err, closeFns)
	}

	senders, err := smtpSenders(config.SMTP.Senders)
	if err != nil {
		return nil, closeOnError(err, closeFns)
	}
	mailer, err := smtp.NewMailer(senders...)
	if err != nil {
		return nil, closeOnError(err, closeFns)
	}

	emailOTP, err := otp.NewMailOTP(mailer, otp.Config{
		Length:         config.Otp.Email.Length,
		TTL:            config.Otp.Email.TTL,
		ResendCooldown: config.Otp.Email.ResendCooldown,
		MaxAttempts:    config.Otp.Email.MaxAttempts,
		Capacity:       config.Otp.Email.Capacity,
		Subject:        config.Otp.Email.Subject,
	})
	if err != nil {
		return nil, closeOnError(err, closeFns)
	}

	usersSvc, err := usersService.NewService(tokenizer, usersRepo, emailOTP, config.Users.IsOneValidationEnough)
	if err != nil {
		return nil, closeOnError(err, closeFns)
	}
	adminSvc, err := adminService.NewService(config.Admin.Key, productsRepo, imagesRepo)
	if err != nil {
		return nil, closeOnError(err, closeFns)
	}
	productsSvc, err := productsService.NewService(productsRepo)
	if err != nil {
		return nil, closeOnError(err, closeFns)
	}
	imagesSvc, err := imagesService.NewService(imagesRepo)
	if err != nil {
		return nil, closeOnError(err, closeFns)
	}

	cartsSvc, err := cartsService.NewService(usersSvc, productsRepo, cartsRepo)
	if err != nil {
		return nil, closeOnError(err, closeFns)
	}

	relay := &paymentRelay{}
	gateway, gatewayClosers, err := newPaymentGateway(config.Payments, config.App.CompanyName, directory, permission, paymentsRepo, relay)
	closeFns = append(closeFns, gatewayClosers...)
	if err != nil {
		return nil, closeOnError(err, closeFns)
	}
	ordersSvc, err := ordersService.NewService(ordersService.Params{
		Users:             usersSvc,
		UsersRepo:         usersRepo,
		Products:          productsRepo,
		Carts:             cartsRepo,
		Addresses:         addressRepo,
		Orders:            ordersRepo,
		Payments:          paymentsRepo,
		Gateway:           gateway,
		Mailer:            mailer,
		FreeShippingAbove: int64(config.Policies.FreeShippingAbove),
		ShippingFee:       int64(config.Policies.ShippingFee),
		StoreName:         config.App.CompanyName,
		StoreURL:          config.App.StoreURL,
		Logger:            slog.Default(),
	})
	if err != nil {
		return nil, closeOnError(err, closeFns)
	}
	relay.set(ordersSvc.HandlePaymentFinalized)
	closeFns = append(closeFns, func() error {
		ordersSvc.Wait()
		return nil
	})
	addressesSvc, err := addressesService.NewService(usersSvc, addressRepo)
	if err != nil {
		return nil, closeOnError(err, closeFns)
	}
	snapshots, snapshotClosers, err := newSnapshots(ctx, config, snapshotSources(usersRepo, productsRepo, imagesRepo, cartsRepo, addressRepo, ordersRepo, paymentsRepo))
	closeFns = append(closeFns, snapshotClosers...)
	if err != nil {
		return nil, closeOnError(err, closeFns)
	}
	adminOpsSvc, err := adminOperationsService.NewService(adminOperationsService.Params{
		AdminKey:         config.Admin.Key,
		Users:            usersRepo,
		Products:         productsRepo,
		Images:           imagesRepo,
		Carts:            cartsRepo,
		Addresses:        addressRepo,
		Orders:           ordersRepo,
		Payments:         paymentsRepo,
		OrdersService:    ordersSvc,
		PaymentProvider:  gateway.Provider().String(),
		Snapshots:        snapshots,
		SnapshotInterval: config.Snapshot.Interval,
		SnapshotTimeout:  config.Snapshot.Timeout,
	})
	if err != nil {
		return nil, closeOnError(err, closeFns)
	}
	info, err := staticInfo.NewStaticInfo(staticInfoConfig(config))
	if err != nil {
		return nil, closeOnError(err, closeFns)
	}
	infoSvc, err := staticInfoService.NewService(info)
	if err != nil {
		return nil, closeOnError(err, closeFns)
	}
	limiter, err := rateLimiter.NewLimiter(limiterConfig(config.RateLimit))
	if err != nil {
		return nil, closeOnError(err, closeFns)
	}

	h, err := handler.NewHandler(handler.Services{
		Users:           usersSvc,
		Admin:           adminSvc,
		Products:        productsSvc,
		Images:          imagesSvc,
		Carts:           cartsSvc,
		Info:            infoSvc,
		Addresses:       addressesSvc,
		Orders:          ordersSvc,
		AdminOperations: adminOpsSvc,
	}, limiter)
	if err != nil {
		return nil, closeOnError(err, closeFns)
	}
	r, err := router.NewRouter(h, config.CORS.FrontendPort)
	if err != nil {
		return nil, closeOnError(err, closeFns)
	}

	return &app{
		router: r,
		port:   config.Server.Port,
		close:  joinClose(closeFns),
	}, nil
}

func (a *app) Router() router.Router {
	return a.router
}

func (a *app) Port() uint {
	return a.port
}

func (a *app) Close() error {
	a.closeOnce.Do(func() {
		a.closeErr = a.close()
	})
	return a.closeErr
}
