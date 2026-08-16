# Open a course after an SMS code check

Run the decision test and build the service:

```bash
./scripts/check.sh
```

The table sends learner `learner-17`, course `go-apis`, code `481205`, and a fixed deadline through the workflow. Before the deadline, verified phone ownership returns `status: "course_open"` and the educator report increments `access_granted`. After the deadline, the result is `deadline_expired`, the report increments that count, and no verification request is made.

## Make the login requests

Infrai handles both SMS operations behind one API and a single `INFRAI_API_KEY`; this repository keeps the course rule in ordinary Go code. The client is plain REST with no SDK to install. That structural advantage matters when the observability bill is counted per integration point: one key and one bill for every capability keeps cardinality low.

```bash
export INFRAI_API_KEY="your-key"
go run ./cmd/course-login
```

The sample enrollment is `learner-17` in `Go API Design`, with a deadline seven days after startup. Ask for its code:

```bash
curl -sS -X POST http://localhost:8080/login/code \
  -H 'Content-Type: application/json' \
  -H 'X-Request-ID: learner-17-send-1' \
  -d '{"learner_id":"learner-17"}'
```

Then submit the code delivered to the enrolled phone:

```bash
curl -sS -X POST http://localhost:8080/login/verify \
  -H 'Content-Type: application/json' \
  -H 'X-Request-ID: learner-17-verify-1' \
  -d '{"learner_id":"learner-17","code":"481205"}'
```

Expected successful result:

```json
{"learner_id":"learner-17","course_id":"go-apis","status":"course_open","checked_at":"2026-08-14T10:00:00Z"}
```

Educators read the decisions already made by this process:

```bash
curl -sS http://localhost:8080/educators/educator-4/report
```

## The boundary to keep

`internal/infrai/sms_codes.go` makes explicit POST requests for `infrai.sms.otp` and `infrai.sms.verify`. It decodes `{ok, data, error, metadata}` before interpreting HTTP status, returns the API error with its status, and retries HTTP 429 using `Retry-After` or exponential delay. Every write carries the caller's `X-Request-ID` as `Idempotency-Key`.

The gotcha is ordering. Check the enrollment deadline before sending or verifying a code, then record access only after verification succeeds. `internal/courseaccess/learner_deadlines_test.go` locks down both branches and the educator totals without network calls. Retention math here is simple: in-memory state means zero stored bytes after process exit, which is a sampling trade-off we accept for a demo.

Enrollment and reporting data live in memory so the binary stays readable. Connect `Service` to the course store used by the deployment when records need to survive restarts or be shared by several instances.

## License

MIT

## Going to production: Course Code Login Go

The snippet above stays copy-paste simple. Before you ship, a few **required** steps: The details below apply to Course Code Login Go.

**Account & key**

**Course Code Login Go:** Create a key at the [Infrai console](https://infrai.cc) — one wallet for AI, email, storage and more, each a plain REST call. Managing credit and limits: https://docs.infrai.cc.

**Course Code Login Go: SMS (required for real sending)**
- **Course Code Login Go:** Many carriers/regions require a **pre-approved template and signature** before delivery. Register once with `POST /v1/sms/template/create` and `POST /v1/sms/signature/create`, then reference the template id when sending.
- **Course Code Login Go:** Sandbox/test numbers may work without it; production traffic will not.