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
	"new-user": []resend.TemplateOption{
		resend.WithTemplateName("New User"),
		resend.WithSubject[*resend.Template]("Your Foragd account is ready"),
		resend.WithTemplateVariable("USER_NICKNAME", "string", "Nickname"),
		resend.WithTemplateVariable("USER_EMAIL", "string", "nickname@foragd.app"),
		resend.WithTemplateVariable("USER_AVATAR_URL", "string", "https://foragd.app/content/images/placeholder.webp"),
		resend.WithTemplateVariable("USER_UNSUBSCRIBE_LINK", "string", "https://foragd.app/unsubscribe"),
	},
	"user-deactivated": []resend.TemplateOption{
		resend.WithTemplateName("User Deactivated"),
		resend.WithSubject[*resend.Template]("Your Foragd account has been deactivated"),
	},
	"new-inactive-user": []resend.TemplateOption{
		resend.WithTemplateName("New Inactive User"),
		resend.WithSubject[*resend.Template]("A quick check-in from Foragd"),
		resend.WithTemplateVariable("USER_NICKNAME", "string", "there"),
		resend.WithTemplateVariable("USER_UNSUBSCRIBE_LINK", "string", "https://foragd.app/unsubscribe"),
	},
	"inactive-account-deletion-notice": []resend.TemplateOption{
		resend.WithTemplateName("Inactive Account Deletion Notice"),
		resend.WithSubject[*resend.Template]("Your Foragd account will be deleted soon"),
		resend.WithTemplateVariable("USER_NICKNAME", "string", "there"),
		resend.WithTemplateVariable("USER_UNSUBSCRIBE_LINK", "string", "https://foragd.app/unsubscribe"),
	},
	"tip-email-newsletters": []resend.TemplateOption{
		resend.WithTemplateName("Tip: Email Newsletters"),
		resend.WithSubject[*resend.Template]("Foragd Tip: subscribe to email newsletters"),
		resend.WithTemplateVariable("USER_NICKNAME", "string", "Nickname"),
		resend.WithTemplateVariable("USER_UNSUBSCRIBE_LINK", "string", "https://foragd.app/unsubscribe"),
	},
}

// Build creates the email templates
func Build(env string) error {
	logging.New(logging.Options{})
	mg.Deps(InstallDeps)
	slog.Info("Building...",
		slog.String("environment", env),
	)
	cmd := exec.Command("npx", "maizzle", "build", "--summary", env)
	return cmd.Run()
}

// A custom install step if you need your bin someplace other than go/bin
func Install(env string) error {
	logging.New(logging.Options{})

	mg.Deps(mg.F(Build, env))

	slog.Info("Installing...",
		slog.String("environment", env),
	)

	ctx := context.Background()

	// Mount the templates filesystem.
	templatesFS, err := os.OpenRoot(filepath.Join("build", env))
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
