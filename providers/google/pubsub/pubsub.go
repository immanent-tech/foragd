// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package pubsub

import (
	"context"
	"fmt"
	"sync"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill-googlecloud/v2/pkg/googlecloud"
	"github.com/ThreeDotsLabs/watermill-http/v2/pkg/http"
	"github.com/ThreeDotsLabs/watermill/components/cqrs"
	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/ThreeDotsLabs/watermill/message/router/middleware"
	slogctx "github.com/veqryn/slog-context"

	gcp "github.com/immanent-tech/foragd/providers/google"
)

type PubSub struct {
	publisher      *googlecloud.Publisher
	eventBus       *cqrs.EventBus
	eventsRouter   *message.Router
	eventProcessor *cqrs.EventProcessor
	subscriber     *googlecloud.Subscriber
	sseRouter      http.SSERouter
}

// New creates a new PubSub using GCP Pub/Sub as a backend.
func New(ctx context.Context) (*PubSub, error) {
	return sync.OnceValues(func() (*PubSub, error) {
		config, err := gcp.LoadConfig()
		if err != nil {
			return nil, fmt.Errorf("load gcp config: %w", err)
		}

		logger := watermill.NewSlogLogger(slogctx.FromCtx(ctx))

		publisher, err := googlecloud.NewPublisher(
			googlecloud.PublisherConfig{
				ProjectID: config.ProjectID,
			},
			logger,
		)
		if err != nil {
			return nil, fmt.Errorf("create publisher: %w", err)
		}

		eventBus, err := cqrs.NewEventBusWithConfig(
			publisher,
			cqrs.EventBusConfig{
				GeneratePublishTopic: func(params cqrs.GenerateEventPublishTopicParams) (string, error) {
					return params.EventName, nil
				},
				Marshaler: cqrs.JSONMarshaler{},
				Logger:    logger,
			},
		)
		if err != nil {
			return nil, fmt.Errorf("create event bus: %w", err)
		}

		router, err := message.NewRouter(message.RouterConfig{}, logger)
		if err != nil {
			return nil, fmt.Errorf("create router: %w", err)
		}
		router.AddMiddleware(middleware.Recoverer)

		eventProcessor, err := cqrs.NewEventProcessorWithConfig(
			router,
			cqrs.EventProcessorConfig{
				GenerateSubscribeTopic: func(params cqrs.EventProcessorGenerateSubscribeTopicParams) (string, error) {
					return params.EventName, nil
				},
				SubscriberConstructor: func(params cqrs.EventProcessorSubscriberConstructorParams) (message.Subscriber, error) {
					return googlecloud.NewSubscriber(
						googlecloud.SubscriberConfig{
							ProjectID: config.ProjectID,
							GenerateSubscriptionName: func(topic string) string {
								return fmt.Sprintf("%v_%v", topic, params.HandlerName)
							},
						},
						logger,
					)
				},
				Marshaler: cqrs.JSONMarshaler{},
				Logger:    logger,
			},
		)
		if err != nil {
			return nil, fmt.Errorf("create event processor: %w", err)
		}

		subscriber, err := googlecloud.NewSubscriber(
			googlecloud.SubscriberConfig{
				ProjectID: config.ProjectID,
				GenerateSubscriptionName: func(topic string) string {
					return fmt.Sprintf("%v_%v", topic, watermill.NewShortUUID())
				},
			},
			logger,
		)
		if err != nil {
			return nil, fmt.Errorf("create subscriber: %w", err)
		}

		sseRouter, err := http.NewSSERouter(
			http.SSERouterConfig{
				UpstreamSubscriber: subscriber,
				Marshaler:          http.StringSSEMarshaler{},
			},
			logger,
		)
		if err != nil {
			return nil, fmt.Errorf("create sse router: %w", err)
		}

		return &PubSub{
			publisher:      publisher,
			eventBus:       eventBus,
			eventProcessor: eventProcessor,
			subscriber:     subscriber,
			sseRouter:      sseRouter,
		}, nil
	})()
}

func (pb *PubSub) AddHandlers(handlers ...cqrs.EventHandler) error {
	if err := pb.eventProcessor.AddHandlers(handlers...); err != nil {
		return fmt.Errorf("add event handlers: %w", err)
	}
	return nil
}

func (pb *PubSub) StartEventsRouter(ctx context.Context) error {
	return pb.eventsRouter.Run(ctx)
}

func (pb *PubSub) StartSSERouter(ctx context.Context) error {
	return pb.sseRouter.Run(ctx)
}
