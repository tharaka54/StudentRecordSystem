# Verification and acceptance checks

## Automated Go tests

```powershell
go test ./... -v
```

Tests create their own temporary SQLite database. They do not use or overwrite `data/student_records.db`.

The suite covers:

- Admin and Student login and unauthenticated access.
- Student A/Student B separation, direct URL access attempts, rejected ID parameters, blocked administrative lists and writes.
- Admin-only dashboard and lookup data; password hashes absent from account responses.
- Database role, foreign-key, student-link and uniqueness constraints.
- Server-side search, status filters, pagination and invalid page sizes.
- Course/unit/student/enrolment/result creation, updates, deletion and duplicate rejection.
- Rejection of cross-course enrolments, out-of-range marks and results on withdrawn enrolments.
- Unequal-credit WAM, failed and zero marks, no-result WAM, grade boundaries and result-deletion recalculation.
- Result deletion reopening the enrolment; enrolment deletion cascading to its result.
- Student password changes, Admin resets, disabled accounts, session invalidation, logout and POST-only logout.
- Persistence of records, password hashes and account links after reopening the database.
- PBKDF2 password verification and a known PBKDF2-SHA256 test vector.

## Browser workflow checks

`tools/browser-smoke.mjs` uses Node.js built-ins and Chrome's local debugging protocol. It needs Node.js 22+ and an installed Google Chrome, but no npm packages.

Use a **dedicated sample database** for this test. It creates and deletes fictional records with `QA-` identifiers and expects the seeded data. Do not point it at a database containing your own records.

Terminal 1, from the project folder:

```powershell
go run . -demo -addr 127.0.0.1:8082 -db data\browser-test.db
```

Terminal 2, start an isolated headless browser profile:

```powershell
& 'C:\Program Files\Google\Chrome\Application\chrome.exe' --headless=new --disable-gpu --no-first-run --remote-debugging-port=9227 --user-data-dir="$env:TEMP\student-records-browser-test" about:blank
```

Terminal 3, from the project folder:

```powershell
$env:APP_URL = 'http://127.0.0.1:8082'
node tools\browser-smoke.mjs
```

The test exercises Admin and Student pages, creates a course/unit/student/enrolment/result/account through real forms, edits the student, changes a student's password, searches and deletes the test records, and checks mobile navigation and horizontal page overflow. It explicitly verifies that changing the selected student rebuilds the Unit dropdown with actual selectable options and clears an incompatible previous selection. It fails on unhandled browser JavaScript exceptions.

Screenshots are saved under `docs/screenshots`. Stop the test server and the isolated test browser after testing.

## Verification record

On 12 October 2026, the automated Go suite and the browser workflow suite were run successfully on Windows. The screenshots show the running application with fictional sample data. Desktop screenshots use 1440×1000; the mobile screenshot uses a 390×844 viewport. The application was checked in Chrome headless; this does not establish compatibility with every browser or device.

The final Assignment 2 version was verified again on 13 October 2026 using Go 1.27.2 on Windows amd64. `go test ./... -v -count=1 -cover` passed with 64.6% root-package statement coverage and 62.5% language-demonstration coverage. `go vet ./...` produced no diagnostics. The browser workflow passed, including server rejection of mark 101 with browser validation bypassed and confirmation that the previously saved 84.5 remained unchanged. Screenshots 08 and 09 record the saved result and rejection.

New tests cover typed JSON null/empty-array contracts, cancelled request contexts, structural-interface substitutes, wrapped missing-student errors, 24 concurrent real HTTP requests for two student accounts, and summary consistency while results are updated. The tests use temporary databases. These concurrency checks do not establish race freedom; the race detector was not run because a suitable Windows C toolchain was not available.

Recorded logs are in `docs/verification`. A short `BenchmarkAcademicSummary` run over four seeded enrolments and two marks measured 12,393,610 ns/op (12.39 ms), 7,237 B/op and 187 allocs/op. It used an Intel i5-11400H and a 300 ms requested benchmark window, completing 42 iterations. It excludes setup, authentication, network and browser work and is not a load test or cross-language comparison.

```powershell
go run ./cmd/language-demo
go test -run "^$" -bench "^BenchmarkAcademicSummary$" -benchtime=300ms -benchmem
```

## Manual acceptance script

| Action | Expected result |
| --- | --- |
| Sign in as Admin | Administrative navigation and aggregate statistics |
| Add a course, then a unit assigned to it | Both records persist and are searchable |
| Add a student assigned to that course | Student appears in list and academic summary |
| Add enrolment, initially without selecting a student | Unit selector disabled with a clear prompt |
| Select a student | Only that student's course units appear |
| Switch to a student on another course | Unit options change and incompatible selection clears |
| Select a student whose course has no units | Clear message explains why no units are available |
| Save enrolment twice for the same period/unit/student | Duplicate is rejected |
| Save a mark of 85 | Grade HD; enrolment Completed; WAM recalculates |
| Save a mark outside 0–100 | Validation rejects it |
| Create a Student account linked to the student | Student can sign in and see only their own information |
| Change own password | Existing sessions end; new password works |
| Admin resets or disables the account | Old session stops working; disabled account cannot log in |
| Delete a referenced course | Clear relationship error; data retained |
| Delete a result | Enrolment reopens and WAM recalculates |
| Close and reopen server | Academic records persist; sign-in required again |
| Use a narrow browser window | Navigation collapses; forms fit; wide tables scroll within their panels |

Testing does not constitute a load test, independent security audit or verification of official academic policy.
