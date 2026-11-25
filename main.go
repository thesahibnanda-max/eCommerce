package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	backendDependencyInjection "github.com/thesahibnanda-max/eCommerce/backend/package/dependency_injection"
	frontendDependencyInjection "github.com/thesahibnanda-max/eCommerce/frontend/package/dependency_injection"
	"golang.org/x/sync/errgroup"

	dotenv "github.com/joho/godotenv"
)

func main() {

	if err := parseAndSetEnvFile(".env"); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() (err error) {
	signalCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	port, singlePort, err := singlePortFromEnv()
	if err != nil {
		return err
	}

	backendApp, err := backendDependencyInjection.NewApp(signalCtx)
	if err != nil {
		return err
	}
	defer func() {
		err = errors.Join(err, backendApp.Close())
	}()

	frontendApp, err := frontendDependencyInjection.NewApp(signalCtx, singlePort)
	if err != nil {
		return err
	}
	defer func() {
		err = errors.Join(err, frontendApp.Close())
	}()

	if singlePort {
		handler := newSinglePortHandler(backendApp.Router().Handler(), frontendApp.Router().Handler())
		return serveSinglePort(signalCtx, port, handler)
	}

	group, ctx := errgroup.WithContext(signalCtx)
	group.Go(func() error {
		return <-backendApp.Router().ListenAndServeOnPort(ctx, backendApp.Port())
	})
	group.Go(func() error {
		return <-frontendApp.Router().ListenAndServeOnPort(ctx, frontendApp.Port())
	})
	return group.Wait()
}

func parseAndSetEnvFile(filePath string) error {
	filePath = strings.TrimSpace(filePath)
	if filePath == "" {
		return nil
	}

	respBytes, err := os.ReadFile(filePath)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to read env file: %w", err)
	}

	envVarMap, err := dotenv.UnmarshalBytes(respBytes)
	if err != nil {
		return fmt.Errorf("failed to unmarshal env file: %w", err)
	}

	for key, value := range envVarMap {
		if _, exists := os.LookupEnv(key); exists {
			continue
		}
		if err := os.Setenv(key, value); err != nil {
			return fmt.Errorf("failed to set env variable %s: %w", key, err)
		}
	}

	return nil
}
