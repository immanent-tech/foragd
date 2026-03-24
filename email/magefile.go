//go:build mage
// +build mage

package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/resend/resend-go/v3"

	"github.com/immanent-tech/foragd/logging"
	resendext "github.com/immanent-tech/foragd/providers/resend"

	"github.com/magefile/mage/mg"
)

var templates = map[string]*resend.UpdateTemplateRequest{
	"new-inactive-user.html": &resend.UpdateTemplateRequest{
		Name:    "New Inactive User",
		Subject: "A quick check-in from Foragd",
	},
}

type config struct {
	templates    *os.Root
	apiKey       string
	replyToEmail string
}

// Build creates the email templates
func Build() error {
	mg.Deps(InstallDeps)
	fmt.Println("Building...")
	cmd := exec.Command("npx", "maizzle", "build", "production")
	return cmd.Run()
}

// A custom install step if you need your bin someplace other than go/bin
func Install() error {
	logging.New(logging.Options{})

	mg.Deps(Build)
	fmt.Println("Installing...")

	ctx := context.Background()

	cfg, err := setupConfig()
	if err != nil {
		return fmt.Errorf("setup config: %w", err)
	}
	defer cfg.templates.Close()

	for filename, template := range templates {
		data, err := cfg.templates.ReadFile(filename)
		template.Html = string(data)
		template.From = cfg.replyToEmail
		if err != nil {
			slog.Warn("Failed to read template.",
				slog.String("file", filename),
				slog.Any("error", err),
			)
		}
		alias := strings.TrimSuffix(filename, filepath.Ext(filename))
		if err := resendext.UpdateTemplate(ctx, alias, template); err != nil {
			slog.Warn("Failed to update template.",
				slog.String("file", filename),
				slog.Any("error", err),
			)
		} else {
			slog.Info("Template updated.",
				slog.String("id", alias))
		}
	}

	return nil
}

// Manage your deps, or running package managers.
func InstallDeps() error {
	fmt.Println("Installing Deps...")
	cmd := exec.Command("npm", "clean-install")
	return cmd.Run()
}

// Clean up after yourself
func Clean() {
	fmt.Println("Cleaning...")
	os.RemoveAll("MyApp")
}

func openEmailTemplatesFS() (*os.Root, error) {
	root, err := os.OpenRoot("build/production")
	if err != nil {
		return nil, fmt.Errorf("open templates directory: %w", err)
	}
	return root, nil
}

func setupConfig() (*config, error) {
	var err error

	cfg := config{}

	cfg.apiKey = os.Getenv("FORAGD_RESEND_APIKEY")
	if cfg.apiKey == "" {
		return nil, errors.New("no api key found")
	}
	cfg.replyToEmail = os.Getenv("FORAGD_RESEND_CATCHALLEMAIL")
	if cfg.replyToEmail == "" {
		return nil, errors.New("no reply to email set")
	}

	cfg.templates, err = openEmailTemplatesFS()
	if err != nil {
		return nil, fmt.Errorf("get templates: %w", err)
	}

	return &cfg, nil
}
