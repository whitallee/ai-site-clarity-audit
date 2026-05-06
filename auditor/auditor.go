package auditor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"strings"
	"sync"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"

	"ai-site-clarity-audit/scraper"
)

type AuditResult struct {
	SiteURL      string   `json:"site_url"`
	WhatItDoes   string   `json:"what_it_does"`
	ClarityScore int      `json:"clarity_score"`
	Suggestions  []string `json:"suggestions"`
}

var (
	client     *anthropic.Client
	clientOnce sync.Once
)

func getClient() *anthropic.Client {
	clientOnce.Do(func() {
		c := anthropic.NewClient()
		client = &c
	})
	return client
}

func Audit(ctx context.Context, scraped *scraper.Result) (*AuditResult, error) {
	prompt := fmt.Sprintf(`Analyze this website and return a JSON clarity report. No markdown, no code fences — raw JSON only.

URL: %s
Title: %s
Meta Description: %s
H1 Tags: %v
H2 Tags: %v
Body Text (excerpt): %s

Evaluate how clearly this site communicates its value proposition to a first-time visitor.

Return exactly this structure:
{
  "what_it_does": "<1-2 sentences describing what this business does, written as if explaining to someone who has never seen the site>",
  "clarity_score": <integer 1-10>,
  "suggestions": [
    "<specific, actionable suggestion to improve messaging clarity>",
    "<specific, actionable suggestion to improve messaging clarity>",
    "<specific, actionable suggestion to improve messaging clarity>"
  ]
}

Scoring guide:
- 9-10: Value prop is immediately clear from the headline alone; zero ambiguity
- 7-8: Clear within a few seconds; minor wording improvements possible
- 5-6: Requires effort to understand; jargon or vagueness present
- 3-4: Confusing; visitor must dig to understand what the business does
- 1-2: Nearly impossible to determine the business purpose from the page`,
		scraped.URL, scraped.Title, scraped.Description,
		scraped.H1s, scraped.H2s, scraped.BodyText,
	)

	msg, err := getClient().Messages.New(ctx, anthropic.MessageNewParams{
		Model:     anthropic.ModelClaudeSonnet4_6,
		MaxTokens: 1024,
		System: []anthropic.TextBlockParam{{
			Text: "You are an expert in marketing and messaging clarity. Your job is to evaluate how clearly a website communicates its value proposition. Respond only with valid JSON — no markdown, no code blocks, no prose.",
		}},
		Messages: []anthropic.MessageParam{{
			Role: anthropic.MessageParamRoleUser,
			Content: []anthropic.ContentBlockParamUnion{{
				OfText: &anthropic.TextBlockParam{Text: prompt},
			}},
		}},
	})
	if err != nil {
		return nil, fmt.Errorf("anthropic API: %w", err)
	}

	raw := msg.Content[0].Text
	raw = strings.TrimPrefix(raw, "```json")
	raw = strings.TrimPrefix(raw, "```")
	raw = strings.TrimSuffix(raw, "```")
	raw = strings.TrimSpace(raw)

	var result AuditResult
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		return nil, fmt.Errorf("parse response: %w\nraw: %s", err, raw)
	}
	result.SiteURL = scraped.URL
	return &result, nil
}

var pdfTemplate = template.Must(template.New("pdf").Parse(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<style>
  *, *::before, *::after { box-sizing: border-box; margin: 0; padding: 0; }
  body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif; color: #1a1a2e; padding: 40px; font-size: 14px; line-height: 1.6; }
  header { border-bottom: 3px solid #2563eb; padding-bottom: 16px; margin-bottom: 24px; }
  header h1 { font-size: 22px; color: #2563eb; }
  header p { color: #64748b; font-size: 13px; margin-top: 4px; }
  .score-block { display: flex; align-items: center; gap: 24px; background: #f0f4ff; border-radius: 8px; padding: 20px; margin-bottom: 24px; }
  .score-number { font-size: 56px; font-weight: 700; color: #2563eb; line-height: 1; min-width: 80px; text-align: center; }
  .score-label { font-size: 11px; font-weight: 600; text-transform: uppercase; letter-spacing: 0.06em; color: #94a3b8; margin-top: 4px; text-align: center; }
  .section { border: 1px solid #e2e8f0; border-radius: 8px; padding: 20px; margin-bottom: 16px; }
  .section h2 { font-size: 13px; font-weight: 600; text-transform: uppercase; letter-spacing: 0.05em; color: #64748b; margin-bottom: 10px; }
  .section p { color: #334155; }
  ul { padding-left: 18px; }
  li { margin-bottom: 6px; color: #334155; }
</style>
</head>
<body>
  <header>
    <h1>Messaging Clarity Report</h1>
    <p>{{.SiteURL}}</p>
  </header>

  <div class="score-block">
    <div>
      <div class="score-number">{{.ClarityScore}}</div>
      <div class="score-label">out of 10</div>
    </div>
    <div>
      <strong>Clarity Score</strong>
      <p style="margin-top:6px;">{{.WhatItDoes}}</p>
    </div>
  </div>

  <div class="section">
    <h2>Suggestions to Improve Clarity</h2>
    <ul>{{range .Suggestions}}<li>{{.}}</li>{{end}}</ul>
  </div>
</body>
</html>`))

func RenderPDF(ctx context.Context, result *AuditResult) ([]byte, error) {
	var buf bytes.Buffer
	if err := pdfTemplate.Execute(&buf, result); err != nil {
		return nil, fmt.Errorf("template: %w", err)
	}

	allocCtx, cancel := chromedp.NewExecAllocator(context.Background(),
		append(chromedp.DefaultExecAllocatorOptions[:],
			chromedp.Flag("headless", true),
			chromedp.Flag("disable-gpu", true),
			chromedp.Flag("no-sandbox", true),
			chromedp.Flag("disable-dev-shm-usage", true),
		)...,
	)
	defer cancel()

	chromCtx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

	htmlContent := buf.String()
	var pdfBuf []byte

	if err := chromedp.Run(chromCtx,
		chromedp.Navigate("about:blank"),
		chromedp.ActionFunc(func(ctx context.Context) error {
			frameTree, err := page.GetFrameTree().Do(ctx)
			if err != nil {
				return err
			}
			return page.SetDocumentContent(frameTree.Frame.ID, htmlContent).Do(ctx)
		}),
		chromedp.ActionFunc(func(ctx context.Context) error {
			var err error
			pdfBuf, _, err = page.PrintToPDF().WithPrintBackground(true).Do(ctx)
			return err
		}),
	); err != nil {
		return nil, fmt.Errorf("chromedp PDF: %w", err)
	}

	return pdfBuf, nil
}
