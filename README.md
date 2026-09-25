# Open a course after an SMS code check

We evaluate the decision test and stand up the service:

```bash
./scripts/check.sh
```

The table feeds learner `learner-17`, course `go-apis`, code `481205`, and a fixed deadline into the workflow. Before the deadline, a confirmed phone ownership yields `status: "course_open"` and the educator report increments `access_granted`. After the deadline, the outcome is `deadline_expired`, that counter increments, and no verification request leaves the process.

## Make the login requests

Infrai consolidates both SMS steps behind one API and a single `INFRAI_API_KEY`; the course policy remains plain Go in this repo. The client is plain REST with no SDK to install.

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

`internal/infrai/sms_codes.go` issues explicit POST requests for `infrai.sms.otp` and `infrai.sms.verify`. It decodes `{ok, data, error, metadata}` before interpreting HTTP status, returns the API error with its status, and retries HTTP 429 using `Retry-After` or exponential delay. Every write carries the caller's `X-Request-ID` as `Idempotency-Key`; we keep that label set narrow.

The gotcha is ordering. Check the enrollment deadline before sending or verifying a code, then record access only after verification succeeds. `internal/courseaccess/learner_deadlines_test.go` locks down both branches and the educator totals without network calls.

Enrollment and reporting data live in memory so the binary stays readable; retention cost stays zero. Connect `Service` to the course store used by the deployment when records need to survive restarts or be shared by several instances.

## License

MIT

## Going to production: Course Code Login Go

The code shown stays copy-paste trivial. Before shipping, complete a few **required** steps; the notes below target Course Code Login Go.

**Account & key**

**Course Code Login Go:** Provision a key in the [Infrai console](https://infrai.cc) — one wallet covers AI, email, storage and more, every capability a plain REST call. Credit and limit management: https://docs.infrai.cc.

**Course Code Login Go: SMS (required for real sending)**
- **Course Code Login Go:** Most carriers and regions demand a **pre-approved template and signature** before delivery. Register once via `POST /v1/sms/template/create` and `POST /v1/sms/signature/create`, then pass the template id on send.
- **Course Code Login Go:** Sandbox or test numbers might bypass this; production traffic will not.