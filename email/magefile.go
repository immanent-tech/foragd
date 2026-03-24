//go:build mage
// +build mage

package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/resend/resend-go/v3"

	"github.com/immanent-tech/foragd/logging"
	resendext "github.com/immanent-tech/foragd/providers/resend"

	"github.com/magefile/mage/mg"
)

var templates = map[string]*resend.UpdateTemplateRequest{
	"new-inactive-user": &resend.UpdateTemplateRequest{
		Name:    "New Inactive User",
		Subject: "A quick check-in from Foragd",
		Variables: []*resend.TemplateVariable{
			{
				Key:           "USER_NICKNAME",
				Type:          "string",
				FallbackValue: "there",
			},
		},
	},
	"new-user": &resend.UpdateTemplateRequest{
		Name:    "New User",
		Subject: "Your Foragd account is ready",
		Variables: []*resend.TemplateVariable{
			{
				Key:           "USER_NICKNAME",
				Type:          "string",
				FallbackValue: "Nickname",
			},
			{
				Key:           "USER_EMAIL",
				Type:          "string",
				FallbackValue: "nickname@foragd.app",
			},
			{
				Key:           "USER_AVATAR_URL",
				Type:          "string",
				FallbackValue: "https://foragd.app/content/images/placeholder.webp",
			},
		},
	},
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

	// Mount the templates filesystem.
	templatesFS, err := os.OpenRoot("build/production")
	if err != nil {
		return fmt.Errorf("open templates directory: %w", err)
	}
	defer templatesFS.Close()

	// Deploy templates.
	for alias, template := range templates {
		// Load the html version of the template.
		htmlData, err := templatesFS.ReadFile(filepath.Join("html", alias+".html"))
		if err != nil {
			slog.Warn("Failed to read html for template.",
				slog.String("file", alias),
				slog.Any("error", err),
			)
			continue
		}
		template.Html = string(htmlData)
		// Load the text version of the template.
		textData, err := templatesFS.ReadFile(filepath.Join("text", alias+".txt"))
		if err != nil {
			slog.Warn("Failed to read text for template.",
				slog.String("file", alias),
				slog.Any("error", err),
			)
			continue
		}
		template.Text = string(textData)

		// Send the updated template data to resend.
		if err := resendext.UpdateTemplate(ctx, alias, template); err != nil {
			slog.Warn("Failed to update template.",
				slog.String("file", alias),
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
