// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package billing

import (
	"context"
	"fmt"
	"log/slog"
	"slices"

	budgets "cloud.google.com/go/billing/budgets/apiv1"
	"cloud.google.com/go/billing/budgets/apiv1/budgetspb"
	"google.golang.org/genproto/googleapis/type/money"

	slogctx "github.com/veqryn/slog-context"

	gcp "github.com/immanent-tech/foragd/providers/google"
	"github.com/immanent-tech/foragd/providers/google/pubsub"
)

// func PrintAllBudgets(ctx context.Context) error {
// 	client, err := budgets.NewBudgetClient(ctx)
// 	if err != nil {
// 		return fmt.Errorf("load budget client: %w", err)
// 	}
// 	defer client.Close()

// 	cfg, err := gcp.LoadConfig()
// 	if err != nil {
// 		return fmt.Errorf("load config: %w", err)
// 	}

// 	billingAccount := "billingAccounts/" + cfg.BillingAccountID

// 	req := &budgetspb.ListBudgetsRequest{
// 		Parent: billingAccount,
// 	}

// 	it := client.ListBudgets(ctx, req)
// 	for {
// 		budget, err := it.Next()
// 		if err == iterator.Done {
// 			break
// 		}
// 		if err != nil {
// 			log.Fatalf("Error iterating budgets: %v", err)
// 		}

// 		printBudgetStatus(budget)
// 	}
// 	return nil
// }

// func printBudgetStatus(budget *budgetspb.Budget) {
// 	fmt.Printf("Budget: %s\n", budget.DisplayName)

// 	// Budget limit
// 	if amount := budget.Amount; amount != nil {
// 		if specified := amount.GetSpecifiedAmount(); specified != nil {
// 			fmt.Printf("  Limit: %d %s\n",
// 				specified.Units,
// 				specified.CurrencyCode,
// 			)
// 		}
// 	}

// 	// Current spend vs thresholds
// 	fmt.Printf("  Threshold Rules:\n")
// 	for _, rule := range budget.ThresholdRules {
// 		pct := rule.ThresholdPercent * 100
// 		basis := rule.SpendBasis.String()
// 		fmt.Printf("    - Alert at %.0f%% of %s\n", pct, basis)
// 	}

// 	fmt.Println()
// }

type Budget struct {
	Name            string `json:"name"`
	DisplayName     string `json:"budgetDisplayName"`
	CurrencyCode    string `json:"currencyCode"`
	Units           int64  `json:"units"`
	ThresholdAlerts []float64
}

type BudgetAlert struct {
	*Budget

	AlertThresholdExceeded float64 `json:"alertThresholdExceeded"`
	BudgetAmount           float64 `json:"budgetAmount"`
	CostAmount             float64 `json:"costAmount"`
}

// func listenForAlerts(projectID, subscriptionID string) {
// 	ctx := context.Background()
// 	client, _ := pubsub.NewClient(ctx, projectID)
// 	defer client.Close()

// 	client.Subscriber(subscriptionID).Receive(ctx, func(ctx context.Context, msg *pubsub.Message) {
// 		var alert BudgetAlert
// 		if err := json.Unmarshal(msg.Data, &alert); err != nil {
// 			log.Printf("Failed to parse alert: %v", err)
// 			msg.Nack()
// 			return
// 		}

// 		pct := (alert.CostAmount / alert.BudgetAmount) * 100
// 		fmt.Printf("⚠️  Budget Alert: %s\n", alert.DisplayName)
// 		fmt.Printf("   Spent: $%.2f / $%.2f (%.1f%%)\n",
// 			alert.CostAmount, alert.BudgetAmount, pct)

// 		msg.Ack()
// 	})
// }

// CreateBudget creates a new budget with the given information. It will return the newly created budget resource id or
// a non-nil error if creation failed.
func CreateBudget(ctx context.Context, budget *Budget) (string, error) {
	client, err := budgets.NewBudgetClient(ctx)
	if err != nil {
		return "", fmt.Errorf("load budget client: %w", err)
	}
	defer client.Close()

	cfg, err := gcp.LoadConfig()
	if err != nil {
		return "", fmt.Errorf("load config: %w", err)
	}

	// Add any threshold alert rules.
	thresholdRules := make([]*budgetspb.ThresholdRule, 0, len(budget.ThresholdAlerts))
	for threshold := range slices.Values(budget.ThresholdAlerts) {
		thresholdRules = append(thresholdRules, &budgetspb.ThresholdRule{ThresholdPercent: threshold})
	}

	// Check and create a pubsub topic for alerts if needed.
	topic := "projects/" + cfg.ProjectID + "/topics/billing-alerts"
	topicExists, err := pubsub.TopicExists(ctx, topic)
	if err != nil {
		return "", fmt.Errorf("check billing pubsub topic: %w", err)
	}
	if !topicExists {
		if err := pubsub.CreateTopic(ctx, topic); err != nil {
			return "", fmt.Errorf("create pubsub topic: %w", err)
		}
	}

	req := &budgetspb.CreateBudgetRequest{
		Parent: cfg.GetBillingAccountName(),
		Budget: &budgetspb.Budget{
			Name:        cfg.GetBillingAccountName() + "/budgets/" + budget.Name,
			DisplayName: budget.DisplayName,
			BudgetFilter: &budgetspb.Filter{
				Projects: []string{"projects/" + cfg.ProjectID},
			},
			Amount: &budgetspb.BudgetAmount{
				BudgetAmount: &budgetspb.BudgetAmount_SpecifiedAmount{
					SpecifiedAmount: &money.Money{
						CurrencyCode: budget.CurrencyCode,
						Units:        budget.Units, // $500 limit
					},
				},
			},
			ThresholdRules: thresholdRules,
			// Send alerts to billing admins + Pub/Sub
			NotificationsRule: &budgetspb.NotificationsRule{
				PubsubTopic:                  topic,
				EnableProjectLevelRecipients: true,
				SchemaVersion:                "1.0",
			},
		},
	}

	newBudget, err := client.CreateBudget(ctx, req)
	if err != nil {
		return "", fmt.Errorf("create budget: %w", err)
	}

	slogctx.FromCtx(ctx).Debug("New budget created.",
		slog.String("budget_name", newBudget.GetDisplayName()),
		slog.String("budget_id", newBudget.GetName()))
	return newBudget.GetName(), nil
}
