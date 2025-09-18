// Copyright 2025 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package main

import (
	"github.com/pulumi/pulumi-digitalocean/sdk/v4/go/digitalocean"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		_, err := digitalocean.NewApp(ctx, "golang-sample", &digitalocean.AppArgs{
			Spec: &digitalocean.AppSpecArgs{
				Name:   pulumi.String("Foragr"),
				Region: pulumi.String("ams"),
				Services: digitalocean.AppSpecServiceArray{
					&digitalocean.AppSpecServiceArgs{
						Name:             pulumi.String("go-service"),
						InstanceCount:    pulumi.Int(1),
						InstanceSizeSlug: pulumi.String("apps-s-1vcpu-1gb"),
						Git: &digitalocean.AppSpecServiceGitArgs{
							RepoCloneUrl: pulumi.String("https://github.com/digitalocean/sample-golang.git"),
							Branch:       pulumi.String("main"),
						},
					},
				},
			},
		})
		if err != nil {
			return err
		}
		return nil
	})
}
