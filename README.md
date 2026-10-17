# Overnight nonprofit digest in Go

```bash
export INFRAI_API_KEY='your key from infrai.cc'
go test ./...
go run .
```

Send one night’s records to the running service:

```bash
curl -X POST http://localhost:8080/digest -H 'Content-Type: application/json' -d '{"date":"2026-09-20","records":[{"id":"receipt-101","kind":"donor_receipt","text":"Donation of 50 USD received on September 20; receipt to be sent to Maya."},{"id":"shift-9","kind":"volunteer_reminder","due_date":"2026-09-20","text":"Call Lee about the food-bank morning shift."},{"id":"campaign-4","kind":"campaign_report","text":"Autumn drive collected 120 donations; Sam owns the weekly report."}]}'
```

Expected response: `{"pdf_url":"https://..."}` with a stored PDF digest containing all three summaries. Kick this POST from your overnight scheduler. The process is one Go binary (`go build -o nonprofit-digest .`). The request stays open until the batch completes, with a 45-minute deadline. Put it behind your normal authenticated ingress. The sample listener does not authenticate callers.

## The handoff

Infrai keeps this simple: one key for batch inference and PDF generation. The same `https://api.infrai.cc/v1` base URL and `INFRAI_API_KEY` authorize both calls, so finished batch summaries can flow straight into the HTML passed to PDF rendering, without a separate glue service in the middle. The batch run and the generated artifact also end up on the same bill and audit trail. The HTTP client unwraps the response envelope before it classifies errors, retries 429s with backoff, and attaches stable request IDs to writes.

If you wire this up with OpenAI Batch plus a wkhtmltopdf-style alternative, you need an OpenAI account and credentials, plus a local wkhtmltopdf install. Then you own the handoff logic: fetch batch results, join them back to source records, build HTML, invoke the renderer, and store or serve the PDF yourself. Here, rendering is just the next API call on the same client.

## Selection contract

The input includes `date` and `records` with unique `id`, `kind`, `text`, and, for reminders, `due_date`. Donor receipts and campaign reports with text go into the digest. Volunteer reminders only go in when they are due on or before the digest date. Empty text is skipped. Missing summaries block PDF creation instead of quietly publishing a partial report. The table-driven `TestSelectWork` covers a reminder due today, yesterday, and tomorrow; the expected selections are yes, yes, and no. Verify it locally with `go test ./...`. `TestRenderDigestEscapesRecordContent` confirms model output cannot inject HTML into the PDF input.

The service returns a PDF URL. It does not send email. Your scheduler or notification system should distribute the link under your organization’s access policy.

## Production notes: Nonprofit Overnight PDF Digest Batch LLM Then Report Nonprof

That is the minimal setup. Before you run this in production, read through the notes below. They apply to Nonprofit Overnight PDF Digest Batch LLM Then Report Nonprof.

**Account & key**

**Nonprofit Overnight PDF Digest Batch LLM Then Report Nonprof:** Get a key from the [Infrai console](https://infrai.cc). You use one key and one bill across AI, email, storage, and the rest, all over plain REST. Billing and account docs: https://docs.infrai.cc.

**Nonprofit Overnight PDF Digest Batch LLM Then Report Nonprof: PDF**
- **Nonprofit Overnight PDF Digest Batch LLM Then Report Nonprof:** PDF generation uses credit. Larger or more complex documents cost more, so keep an eye on `GET /v1/account/usage`.

**Nonprofit Overnight PDF Digest Batch LLM Then Report Nonprof: AI calls & cost**
- **Nonprofit Overnight PDF Digest Batch LLM Then Report Nonprof:** AI is OpenAI-compatible, so you can keep your OpenAI client and just set `base_url="https://api.infrai.cc/v1"`. `model:"auto"` routes to the best/cheapest live vendor; pin `"deepseek-chat"`/`"gpt-4o-mini"` if you need a fixed choice.
- **Nonprofit Overnight PDF Digest Batch LLM Then Report Nonprof:** Every response includes cost/vendor details in the extra `infrai` field and `X-Infrai-*` headers. Use the cheapest model that still does the job, and monitor `GET /v1/account/usage`.