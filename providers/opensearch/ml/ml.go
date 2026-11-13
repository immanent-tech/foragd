/*
 * Copyright (c) 2026 Immanent Tech
 * SPDX-License-Identifier: AGPL-3.0-or-later
 */

package ml

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/opensearch-project/opensearch-go/v5/opensearchapi"
	"github.com/opensearch-project/opensearch-go/v5/plugins/ml"
	slogctx "github.com/veqryn/slog-context"

	"github.com/immanent-tech/go-base/config"
	"github.com/immanent-tech/go-base/validation"

	"github.com/immanent-tech/foragd/providers/opensearch"
)

const (
	configEnvPrefix = "OPENSEARCH_ML_"
)

type Config struct {
	ConnectorID  string  `koanf:"connectorid"  validate:"required"`
	ModelID      string  `koanf:"modelid"      validate:"required"`
	Name         string  `koanf:"name"         validate:"required"`
	Description  string  `koanf:"description"  validate:"required"`
	Version      int     `koanf:"version"      validate:"required,gt=0"`
	Protocol     string  `koanf:"protocol"     validate:"required,oneof=http"`
	Endpoint     string  `koanf:"endpoint"     validate:"required,hostname_port"`
	Model        string  `koanf:"model"        validate:"required"`
	AccessKey    *string `koanf:"accesskey"    validate:"omitnil"`
	SecretKey    *string `koanf:"secretkey"    validate:"omitnil"`
	SessionToken *string `koanf:"sessiontoken" validate:"omitnil"`
}

func loadConfig() (*Config, error) {
	var cfg Config
	if err := config.Load(configEnvPrefix, &cfg); err != nil {
		return nil, fmt.Errorf("load server environment: %w", err)
	}
	if err := validation.Validate.Struct(cfg); err != nil {
		return nil, fmt.Errorf("validate config: %w", err)
	}
	return &cfg, nil
}

type Client struct {
	*ml.Client

	cfg *Config
}

// NewClient creates a new OpenSearch ML client with the config defined in the environment.
func NewClient(osc *opensearch.Client) (*Client, error) {
	cfg, err := loadConfig()
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}
	return &Client{Client: ml.NewClient(osc.Client.Client), cfg: cfg}, nil
}

// Configure performs initial configuration or updates of the OpenSearch ML config. This includes creating/updating the
// model connector and the models themselves.
func (c *Client) Configure(ctx context.Context) (string, string, error) {
	// If the model already exists, undeploy it first.
	if exists, err := c.modelExists(ctx, c.cfg.ModelID); err == nil && exists {
		resp, err := c.Model.UndeployModel(ctx, ml.UndeployModelReq{
			ModelID: c.cfg.ModelID,
		})
		if err != nil {
			return "", "", opensearch.HandleError("undeploy model", resp, err)
		}
	}
	// Configure the connector.
	connectorID, err := c.ConfigureMLConnector(ctx)
	if err != nil {
		return "", "", fmt.Errorf("configure connector: %w", err)
	}
	// Configure the model.
	modelID, err := c.ConfigureModel(ctx, connectorID)
	if err != nil {
		return "", "", fmt.Errorf("configure model: %w", err)
	}
	return connectorID, modelID, nil
}

// Connector contains the configuration details for a model connector.
type Connector struct {
	ID          string                     `json:"connector_id"`
	Name        string                     `json:"name"`
	Description string                     `json:"description"`
	Version     int                        `json:"version"`
	Protocol    string                     `json:"protocol"`
	Parameters  ConnectorParameters        `json:"parameters"`
	Credential  opensearchapi.MLCredential `json:"credential"`
	Actions     []opensearchapi.MLAction   `json:"actions"`
}

type ConnectorParameters struct {
	Endpoint string `json:"endpoint"`
	Model    string `json:"model"`
}

// ConfigureMLConnector will either create or update an model connector in the cluster, based on the config read from
// the environment.
func (c *Client) ConfigureMLConnector(ctx context.Context) (string, error) {
	// Set up any specified credentials.
	credentials := opensearchapi.MLCredential{}
	if c.cfg.AccessKey != nil {
		credentials.AccessKey = c.cfg.AccessKey
	}
	if c.cfg.SecretKey != nil {
		credentials.SecretKey = c.cfg.SecretKey
	}
	if c.cfg.SessionToken != nil {
		credentials.SessionToken = c.cfg.SessionToken
	}

	// Generate URL.
	var (
		url     string
		reqBody string
	)
	switch c.cfg.Protocol {
	case "http":
		url = `http://${parameters.endpoint}/v1/embeddings`
		reqBody = `{ "model": "${parameters.model}", "input": ${parameters.input} }`
	}

	// Create connector config.
	connector := Connector{
		ID:          c.cfg.ConnectorID,
		Name:        c.cfg.Name,
		Description: c.cfg.Description,
		Version:     c.cfg.Version,
		Protocol:    c.cfg.Protocol,
		Credential:  credentials,
		Parameters: ConnectorParameters{
			Endpoint: c.cfg.Endpoint,
			Model:    c.cfg.Model,
		},
		Actions: []opensearchapi.MLAction{
			{
				ActionType:          new("predict"),
				Method:              new(http.MethodPost),
				URL:                 &url,
				RequestBody:         &reqBody,
				PreProcessFunction:  new("connector.pre_process.openai.embedding"),
				PostProcessFunction: new("connector.post_process.openai.embedding"),
			},
		},
	}
	body, err := json.Marshal(connector)
	if err != nil {
		return "", fmt.Errorf("marshal connector: %w", err)
	}

	resp, err := c.Connector.GetConnector(ctx, ml.GetConnectorReq{ConnectorID: c.cfg.ConnectorID})
	if err != nil && resp.Inspect().Response.StatusCode != http.StatusNotFound {
		return "", opensearch.HandleError("get connector", resp, err)
	}
	switch resp.Inspect().Response.StatusCode {
	case http.StatusOK:
		// Create connector.
		req := ml.UpdateConnectorReq{
			ConnectorID: c.cfg.ConnectorID,
			BodyReader:  bytes.NewReader(body),
			Header:      opensearch.NewRequestHeaders(ctx),
		}
		resp, err := c.Connector.UpdateConnector(ctx, req)
		if err != nil {
			return "", opensearch.HandleError("update connector", resp, err)
		}
		slogctx.Info(ctx, "Connector updated.", slog.String("connector_id", c.cfg.ConnectorID))
	case http.StatusNotFound:
		// Create connector.
		req := &ml.CreateConnectorReq{
			BodyReader: bytes.NewReader(body),
			Header:     opensearch.NewRequestHeaders(ctx),
		}
		resp, err := c.Connector.CreateConnector(ctx, req)
		if err != nil {
			return "", opensearch.HandleError("create connector", resp, err)
		}
		slogctx.Info(ctx, "Connector created.", slog.String("connector_id", c.cfg.ConnectorID))
	}

	return c.cfg.ConnectorID, nil
}

type Model struct {
	ml.MLRegisterModelBody

	ID *string `json:"model_id,omitempty"`
}

// ConfigureModel will register and deploy a model from the given connector.
func (c *Client) ConfigureModel(ctx context.Context, connectorID string) (string, error) {
	modelID := connectorID + "-model"
	// Create model config.
	model := &Model{
		ID:           &modelID,
		ConnectorID:  &connectorID,
		Name:         "Model " + c.cfg.Model,
		FunctionName: new("remote"),
		Description:  new("Remote model " + c.cfg.Model + " from " + c.cfg.Name),
	}
	body, err := json.Marshal(model)
	if err != nil {
		return "", fmt.Errorf("marshal model: %w", err)
	}

	exists, err := c.modelExists(ctx, c.cfg.ModelID)
	if err != nil {
		return "", fmt.Errorf("check model exists: %w", err)
	}
	if exists {
		// Update existing model.
		resp, err := c.Model.UpdateModel(ctx, ml.UpdateModelReq{
			ModelID:    modelID,
			Header:     opensearch.NewRequestHeaders(ctx),
			BodyReader: bytes.NewBuffer(body),
		})
		if err != nil {
			return "", opensearch.HandleError("update model", resp, err)
		}
		slogctx.Info(
			ctx,
			"Model updated.",
			slog.String("connector_id", c.cfg.ConnectorID),
			slog.String("model_id", modelID),
		)
	} else {
		// Create model.
		registerResp, err := c.Model.RegisterModel(ctx, &ml.RegisterModelReq{
			Header:     opensearch.NewRequestHeaders(ctx),
			BodyReader: bytes.NewReader(body),
		})
		if err != nil {
			return "", opensearch.HandleError("register model", registerResp, err)
		}
		slogctx.Info(
			ctx,
			"Model created.",
			slog.String("connector_id", c.cfg.ConnectorID),
			slog.String("model_id", modelID),
		)
	}

	deployResp, err := c.Model.DeployModel(ctx, ml.DeployModelReq{
		Header:  opensearch.NewRequestHeaders(ctx),
		ModelID: modelID,
	})
	if err != nil {
		return "", opensearch.HandleError("deploy model", deployResp, err)
	}

	return modelID, nil
}

func (c *Client) modelExists(ctx context.Context, modelID string) (bool, error) {
	resp, err := c.Model.GetModel(ctx, ml.GetModelReq{
		Header:  opensearch.NewRequestHeaders(ctx),
		ModelID: modelID,
	})
	if err != nil && resp.Inspect().Response.StatusCode != http.StatusNotFound {
		return false, opensearch.HandleError("get model", resp, err)
	}
	return resp.Inspect().Response.StatusCode == http.StatusOK, nil
}
