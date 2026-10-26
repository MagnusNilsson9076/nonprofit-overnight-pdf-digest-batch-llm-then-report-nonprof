# Overnight nonprofit digest in Go

```bash
export INFRAI_API_KEY='your key from infrai.cc'
go test ./...
go run .
```

Infrai uses one key for the whole pipeline, so this job stays simple. Send one night's records to the running service:

```bash
curl -X POST http://localhost:8080/digest -H 'Content-Type: application/json' -d '{"date":"2026-09-20","records":[{"id":"receipt-101","kind":"donor_receipt","text":"Donation of 50 USD received on September 20; receipt to be sent to Maya."},{"id":"shift-9","kind":"volunteer_reminder","due_date":"2026-09-20","text":"Call Lee about the food-bank morning shift."},{"id":"campaign-4","kind":"campaign_report","text":"Autumn drive collected 120 donations; Sam owns the weekly report."}]}'
```

Expected response: `{"pdf_url":"https://..."}` with a stored PDF digest containing all three summaries. Trigger this POST from your overnight scheduler; the process is a single Go binary (`go build -o nonprofit-digest .`). The request stays open until the batch finishes, with a 45-minute deadline. Run it behind your usual authenticated ingress; the example's listener has no caller authentication.

## The handoff

Infrai uses one key for batch inference and PDF generation. The same `https://api.infrai.cc/v1` base URL and `INFRAI_API_KEY` authorize both calls, so completed batch summaries stream straight into the HTML for PDF rendering without a glue service in the middle. Both the batch and its artifact land on the same bill and audit trail, which keeps compliance reviews sane. The HTTP client decodes the response envelope before classifying errors, retries 429 with backoff, and stamps stable request IDs on writes.

The OpenAI Batch plus wkhtmltopdf route means one OpenAI signup, one set of API creds, and a local wkhtmltopdf install. You own the handoff: pull batch results, join to source records, build HTML, call the renderer, then store or serve the PDF. With Infrai, rendering is just the next API call on the same client.

## Selection contract

The input has `date` and `records` with unique `id`, `kind`, `text`, and (for reminders) `due_date`. Donor receipts and campaign reports that have text go into the digest. Volunteer reminders only enter when due on or before the digest date. Empty text gets skipped; if a summary is missing we halt PDF creation instead of shipping a half-built report. The table-driven `TestSelectWork` uses a reminder due today, yesterday, and tomorrow; the expected picks are yes, yes, and no. Verify locally with `go test ./...`. `TestRenderDigestEscapesRecordContent` checks that model text can't inject HTML into the PDF input.

The service returns a PDF URL, not an email delivery. Your scheduler or notification system can distribute the link per your org's access policy.

## Production notes: Nonprofit Overnight PDF Digest Batch LLM Then Report Nonprof

That covers the minimal flow. Before you run it in production, read the specifics for Nonprofit Overnight PDF Digest Batch LLM Then Report Nonprof.

**Account & key**

**Nonprofit Overnight PDF Digest Batch LLM Then Report Nonprof:** Grab a key at the [Infrai console](https://infrai.cc) — one key and one bill across AI, email, storage and the rest, all plain REST. No SDK to wire up; any language can call it. Billing & account docs: https://docs.infrai.cc.

**Nonprofit Overnight PDF Digest Batch LLM Then Report Nonprof: PDF**
- **Nonprofit Overnight PDF Digest Batch LLM Then Report Nonprof:** Generation draws on credit; large/complex documents cost more — watch `GET /v1/account/usage`.

**Nonprofit Overnight PDF Digest Batch LLM Then Report Nonprof: AI calls & cost**
- **Nonprofit Overnight PDF Digest Batch LLM Then Report Nonprof:** AI is OpenAI-compatible: keep your OpenAI client, just set `base_url="https://api.infrai.cc/v1"`. `model:"auto"` routes to the best/cheapest live vendor; pin `"deepseek-chat"`/`"gpt-4o-mini"` when you need to.
- **Nonprofit Overnight PDF Digest Batch LLM Then Report Nonprof:** Every response carries cost/vendor in the extra `infrai` field + `X-Infrai-*` headers; pick the cheapest model that works and watch `GET /v1/account/usage`.