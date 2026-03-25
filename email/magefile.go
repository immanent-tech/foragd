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

	"github.com/immanent-tech/foragd/logging"
	"github.com/immanent-tech/foragd/providers/resend"

	"github.com/magefile/mage/mg"
)

var templates = map[string][]resend.TemplateOption{
	"new-inactive-user": []resend.TemplateOption{
		resend.WithTemplateName("New Inactive User"),
		resend.WithTemplateSubject("A quick check-in from Foragd"),
		resend.WithTemplateVariable("USER_NICKNAME", "string", "there"),
	},
	"new-user": []resend.TemplateOption{
		resend.WithTemplateName("New User"),
		resend.WithTemplateSubject("Your Foragd account is ready"),
		resend.WithTemplateVariable("USER_NICKNAME", "string", "Nickname"),
		resend.WithTemplateVariable("USER_EMAIL", "string", "nickname@foragd.app"),
		resend.WithTemplateVariable("USER_AVATAR_URL", "string", "https://foragd.app/content/images/placeholder.webp"),
	},
	"user-deactivated": []resend.TemplateOption{
		resend.WithTemplateName("User Deactivated"),
		resend.WithTemplateSubject("Your Foragd account has been deactivated"),
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
	for alias, options := range templates {
		// Load the html version of the template.
		htmlData, err := templatesFS.ReadFile(filepath.Join("html", alias+".html"))
		if err != nil {
			slog.Warn("Failed to read html for template.",
				slog.String("file", alias),
				slog.Any("error", err),
			)
			continue
		}
		options = append(options, resend.WithTemplateHTML(htmlData))
		// Load the text version of the template.
		textData, err := templatesFS.ReadFile(filepath.Join("text", alias+".txt"))
		if err != nil {
			slog.Warn("Failed to read text for template.",
				slog.String("file", alias),
				slog.Any("error", err),
			)
			continue
		}
		options = append(options, resend.WithTemplateText(textData))

		// Send the updated template data to resend.
		if err := resend.UpdateTemplate(ctx, alias, options...); err != nil {
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
