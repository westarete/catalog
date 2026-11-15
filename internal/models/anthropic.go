package models

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

type anthropicProvider struct {
	client anthropic.Client
	model  string
}

// NewAnthropic returns a Provider that calls the Anthropic Messages API. The
// key comes from the caller, never from the environment.
func NewAnthropic(apiKey, model string) Provider {
	return &anthropicProvider{
		client: anthropic.NewClient(option.WithoutEnvironmentDefaults(), option.WithAPIKey(apiKey)),
		model:  model,
	}
}

func (p *anthropicProvider) Complete(ctx context.Context, req Request) (Response, error) {
	resp, err := p.client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     anthropic.Model(p.model),
		MaxTokens: req.MaxTokens,
		System:    []anthropic.TextBlockParam{{Text: req.System}},
		Messages:  []anthropic.MessageParam{anthropic.NewUserMessage(contentBlocks(req)...)},
	})
	if err != nil {
		return Response{}, err
	}
	var out strings.Builder
	for _, block := range resp.Content {
		if block.Type == "text" {
			out.WriteString(block.Text)
		}
	}
	if resp.StopReason == anthropic.StopReasonRefusal {
		return Response{}, fmt.Errorf("%w: model refused (%s): %s", ErrEmptyReply, resp.StopDetails.Category, resp.StopDetails.Explanation)
	}
	return Response{
		Text:         out.String(),
		InputTokens:  resp.Usage.InputTokens,
		OutputTokens: resp.Usage.OutputTokens,
	}, nil
}

// contentBlocks puts the PDF, if any, before the text, as today's tool did.
func contentBlocks(req Request) []anthropic.ContentBlockParamUnion {
	var blocks []anthropic.ContentBlockParamUnion
	if req.PDF != nil {
		blocks = append(blocks, anthropic.NewDocumentBlock(anthropic.Base64PDFSourceParam{
			Data: base64.StdEncoding.EncodeToString(req.PDF),
		}))
	}
	if req.Text != "" {
		blocks = append(blocks, anthropic.NewTextBlock(req.Text))
	}
	return blocks
}
