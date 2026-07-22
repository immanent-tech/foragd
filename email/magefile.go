// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

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

	"github.com/immanent-tech/go-base/logging"
	"github.com/magefile/mage/mg"

	"github.com/immanent-tech/foragd/providers/resend"
)

var templates = map[string][]resend.TemplateOption{

	"new-user": []resend.TemplateOption{
		resend.WithTemplateName("New User"),
		resend.WithSubject[*resend.Template]("Your Foragd account is ready"),
		resend.WithTemplateVariable("USER_NICKNAME", "string", "Nickname"),
	},
	"user-deactivated": []resend.TemplateOption{
		resend.WithTemplateName("User Deactivated"),
		resend.WithSubject[*resend.Template]("Your Foragd account has been deactivated"),
	},
	"new-inactive-user": []resend.TemplateOption{
		resend.WithTemplateName("New Inactive User"),
		resend.WithSubject[*resend.Template]("A quick check-in from Foragd"),
		resend.WithTemplateVariable("USER_NICKNAME", "string", "there"),
	},
	"tip-email-newsletters": []resend.TemplateOption{
		resend.WithTemplateName("Tip: Email Newsletters"),
		resend.WithSubject[*resend.Template]("Foragd Tip: subscribe to email newsletters"),
		resend.WithTemplateVariable("USER_NICKNAME", "string", "there"),
	},
	"subscription-thank-you": []resend.TemplateOption{
		resend.WithTemplateName("Subscription Thank You"),
		resend.WithSubject[*resend.Template]("Thank you for buying a Foragd subscription"),
		resend.WithTemplateVariable("USER_NICKNAME", "string", "there"),
	},
	"trial-checkin": []resend.TemplateOption{
		resend.WithTemplateName("Trial Checkin"),
		resend.WithSubject[*resend.Template]("Checking in about your Foragd trial"),
		resend.WithTemplateVariable("USER_NICKNAME", "string", "there"),
	},
	"trial-expiring": []resend.TemplateOption{
		resend.WithTemplateName("Trial Expiring"),
		resend.WithSubject[*resend.Template]("Your Foragd trial is expiring soon"),
		resend.WithTemplateVariable("USER_NICKNAME", "string", "there"),
	},
	"account-limit-exceeded": []resend.TemplateOption{
		resend.WithTemplateName("Account Limit Exceeded"),
		resend.WithSubject[*resend.Template]("You've exceeded your account limits"),
		resend.WithTemplateVariable("USER_NICKNAME", "string", "there"),
		resend.WithTemplateVariable("LIMIT_NAME", "string", ""),
		resend.WithTemplateVariable("TOTAL", "number", 0),
		resend.WithTemplateVariable("ALLOWED", "number", 0),
	},
}

// Build creates the email templates
func Build() error {
	logging.New()
	slog.Info("Building...")
	cmd := exec.Command("maizzle", "build")
	return cmd.Run()
}

// Install will load the templates into Resend.
func Install() error {
	logging.New()

	mg.Deps(Build)

	slog.Info("Installing...")

	ctx := context.Background()

	// Mount the templates filesystem.
	templatesFS, err := os.OpenRoot(filepath.Join("dist"))
	if err != nil {
		return fmt.Errorf("open templates directory: %w", err)
	}
	defer templatesFS.Close()

	// Deploy templates.
	for alias, options := range templates {
		// Load the html version of the template.
		htmlData, err := templatesFS.ReadFile(alias + ".html")
		if err != nil {
			slog.Warn("Failed to read html for template.",
				slog.String("file", alias),
				slog.Any("error", err),
			)
			continue
		}
		options = append(options, resend.WithTemplateHTML(htmlData))
		// Load the text version of the template.
		textData, err := templatesFS.ReadFile(alias + ".txt")
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
