# Messaging Clarity Analyzer

A web app that takes any URL and tells you how clearly the site communicates its value proposition — with a 1–10 clarity score, a plain-English description of what the business does, and specific suggestions to improve.

---

## What it does

Paste in a URL and get three things:

- **What it does** — 1–2 sentences describing the business, written as if explaining it to someone who has never seen the site
- **Clarity score** — 1–10 rating of how clearly the site communicates its value proposition to a first-time visitor
- **Suggestions** — 2–3 specific, actionable recommendations to improve messaging clarity

Results can be exported as a PDF.

---

## How it's built

**Backend — Go**

- `scraper/` — uses [chromedp](https://github.com/chromedp/chromedp) (headless Chrome) to visit the URL and extract page title, meta description, H1/H2 tags, and body text
- `auditor/` — sends the scraped content to Claude via the Anthropic Go SDK using structured outputs (`OutputConfig` + `JSONOutputFormatParam`) to guarantee the response matches the expected schema; uses chromedp to render the PDF via Chrome's print engine
- `main.go` — lightweight HTTP server using Go's built-in `ServeMux`; results held in-memory keyed by UUID for the PDF download flow

**Frontend — Vanilla JS**

- Single HTML page, no framework, no build step
- Score is color-coded green/amber/red (8–10 / 5–7 / 1–4)
- PDF download hits `GET /api/audit/{id}/pdf`, which streams bytes directly from the server

---

## Running locally

```bash
cp .env.example .env
# add your ANTHROPIC_API_KEY to .env

make run
# → http://localhost:8080
```

Requires Go 1.26+ and Chrome/Chromium installed locally.

---

## Project structure

```
main.go      — HTTP server and route handlers
scraper/     — headless browser scraping
auditor/     — Claude API call, structured output parsing, PDF rendering
static/      — HTML, CSS, JS
Dockerfile   — multi-stage build (Go compiler → minimal Debian + Chromium)
```
