// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package handler

import (
	"encoding/json"

	"github.com/straubt1/terraform-run-task/internal/sdk/api"
)

// TypeTaskResults is the data type used in run task responses.
// const TypeTaskResults = "task-results"

// type callbackResponse struct {
// 	Data api.CallbackData `json:"data"`
// }

// type callbackData struct {
// 	Type       string       `json:"type"`
// 	Attributes api.Response `json:"attributes"`
// }

// Wrapper around api.TaskResponse to build the response in a fluent style.
type CallbackBuilder struct {
	resp api.TaskResponse
}

func NewCallbackBuilder(status api.TaskStatus) *CallbackBuilder {
	return &CallbackBuilder{
		resp: api.TaskResponse{
			Data: api.ResponseData{
				Type: api.TypeTaskResults,
				Attributes: api.ResponseAttributes{
					Status: status,
				},
			},
		},
	}
}

func (cb *CallbackBuilder) WithMessage(message string) *CallbackBuilder {
	cb.resp.Data.Attributes.Message = message
	return cb
}

func (cb *CallbackBuilder) WithUrl(url string) *CallbackBuilder {
	// Causing the Details to be blank??
	cb.resp.Data.Attributes.URL = url
	return cb
}

func (cb *CallbackBuilder) WithRelationships() *CallbackBuilder {
	cb.resp.Data.Relationships = &api.ResponseRelationships{
		Outcomes: api.ResponseOutcomes{
			Data: []api.ResponseOutcome{
				{
					Type: api.TypeTaskResultOutcomes,
					Attributes: api.ResponseOutcomeAttributes{
						OutcomeID:   "outcome-1",
						Description: "Description of outcome 1",
						Tags: api.Tags{
							Status: []api.Tag{
								{
									Label: "info",
									Level: api.TagLevelInfo,
								},
							},
							Severity: []api.Tag{
								{
									Label: "high",
									Level: api.TagLevelError,
								},
							},
							Custom: []api.Tag{
								{
									Label: "custom-tag",
									Level: api.TagLevelNone,
								},
							},
						},
						Body: "# Markdown Formatting Examples\n\n" +
							"## Text Formatting\n" +
							"**Bold text** and *italic text* and ***bold italic***\n" +
							"~~Strikethrough text~~\n" +
							"`Inline code`\n\n" +
							"## Lists\n" +
							"### Unordered List\n" +
							"- Item 1\n" +
							"- Item 2\n" +
							"  - Nested item\n" +
							"  - Another nested item\n\n" +
							"### Ordered List\n" +
							"1. First item\n" +
							"2. Second item\n" +
							"   1. Nested numbered item\n" +
							"   2. Another nested item\n\n" +
							"## Links and Images\n" +
							"[Link text](https://example.com)\n" +
							"![Alt text](https://example.com/image.png)\n\n" +
							"## Code Blocks\n" +
							"```hcl\n" +
							"resource \"random_pet\" \"main\" {\n" +
							"  length = 8\n" +
							"}\n" +
							"```\n\n" +
							"## Blockquotes\n" +
							"> This is a blockquote\n" +
							"> with multiple lines\n\n" +
							"## Tables\n" +
							"| Header 1 | Header 2 | Header 3 |\n" +
							"|----------|----------|----------|\n" +
							"| Cell 1   | Cell 2   | Cell 3   |\n" +
							"| Cell 4   | Cell 5   | Cell 6   |\n\n" +
							"## Horizontal Rule\n" +
							"---\n\n" +
							"## Task Lists\n" +
							"- [x] Completed task\n" +
							"- [ ] Incomplete task\n" +
							"- [ ] Another task\n",
					},
				},
			},
		},
	}
	return cb
}

func (cb *CallbackBuilder) MarshallJSON() ([]byte, error) {
	return json.Marshal(cb.resp)
}

func (cb *CallbackBuilder) GetResponse() api.TaskResponse {
	return cb.resp
}
