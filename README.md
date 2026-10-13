# Student Record Management System

A Go application with separate **Admin** and **Student** portals, SQLite persistence, and an embedded HTML/CSS/JavaScript interface. It extends the earlier project while replacing the shared viewer role with individual student accounts.

## Windows: quickest start

1. Extract the complete ZIP to a writable folder.
2. Double-click **START_WINDOWS.bat** and keep its window open.
3. Open **http://127.0.0.1:8081**.
4. Sign in using the sample accounts below.

The ZIP includes a Windows x64 executable. It contains the interface and schema, so no Node.js, npm, separate web server or SQLite installation is needed. If the executable is absent, the launcher uses the installed Go toolchain instead.

| Role | Username | Initial sample password |
| --- | --- | --- |
| Admin | `admin` | `Admin@12345` |
| Student: Nimal Perera | `s2026001` | `Student@12345` |
| Student: Amara Silva | `s2026002` | `Student@12345` |

The other fictional student accounts use `s2026003` through `s2026008` with the same initial sample password. These are local sample credentials. Change them through Account security / Change password or the Admin's Student accounts page when retaining the database.

The sample database is generated on first launch. A database is not included in the ZIP. Reopening the application retains saved records and passwords; `-demo` never overwrites an existing account database.

For an empty database, run **START_EMPTY_WINDOWS.bat**. Its username is `admin`; the first launch prints a randomly generated password once. Save that password. The empty database is `data/registry.db`, separate from the sample database `data/student_records.db`.

Only one application can listen on port 8081 at a time. Stop the running server with Ctrl+C before opening another launcher. Alternatively, use `-addr 127.0.0.1:8090` from the terminal and open that address.

## Develop in Visual Studio Code

Install Go 1.23 or later and the Go extension for VS Code. The Assignment 2 version was built and tested using Go 1.27.2 on Windows x64 on 13 October 2026. Open the folder containing `go.mod`, then run:

```powershell
go mod download
go test ./... -v
go run . -demo -addr 127.0.0.1:8081
```

Dependencies require an Internet connection on their first download. The SQLite driver is pure Go; no C compiler is needed for a normal build. Stop a running compiled executable before replacing it:

```powershell
go build -o student-records.exe .
```

HTML, CSS, JavaScript and the schema are embedded at build time. Restart `go run` or rebuild the executable after source edits, then refresh the browser. VS Code tasks for running, testing and building are included.

## Admin workflow

1. Create a **Course**, then add **Units** linked to that course.
2. Add a **Student** and assign the student's course.
3. Open **Student accounts**, select the student, review the generated username and set an initial password. A student record does not automatically create an account.
4. Add an **Enrolment** by selecting the student, a unit from the same course, and an academic period. The unit selector is disabled until a student is selected. If the course has no units, the form explains this.
5. Add a **Result** for the enrolment. Saving a mark calculates the grade and marks the enrolment Completed.
6. Open the student's **Academic summary** to review all periods, results, earned credits and WAM.

Search, status/grade/course filters and pagination run against the server's data. Administrators can edit records, reset passwords, disable accounts and delete records subject to relationship checks. Deleting an enrolment also deletes its result after confirmation. Deleting a result returns its enrolment to Enrolled and recalculates WAM.

## Student workflow

Students sign in with their individual usernames. They can view their own dashboard, profile, course, unit enrolments, results and academic summary, and change their own passwords. A password change ends existing sessions and requires a fresh login. Student accounts cannot access administration endpoints or select another student's record through an ID in a URL.

## Academic rules

- One student belongs to one course in this implementation; each unit also belongs to one course.
- Enrolments must match the student's course. Duplicate enrolments for the same student, unit, year and semester are rejected.
- Marks range from 0 to 100. Grades are HD (80+), D (70+), C (60+), P (50+) and F (below 50).
- WAM is `SUM(mark × credit_points) / SUM(credit_points)` over completed, graded enrolments. Failed and zero-mark results count. Withdrawn and ungraded enrolments do not. No graded units means no WAM, displayed as a dash.
- Earned credits count completed units with marks of at least 50.
- Grade thresholds and repeat-enrolment handling are application rules for this project, not a claim about official institutional policy. WAM uses the currently configured unit credit values.
- Student status describes academic registration. Account status separately determines whether the student can sign in, allowing inactive students to access past records if their accounts remain enabled.

## Files and documentation

| File | Purpose |
| --- | --- |
| `main.go` | Server startup, embedded assets, shutdown |
| `auth.go` | Password hashing, sessions, CSRF, role checks |
| `api.go` | Admin resource definitions, validation, CRUD, dashboard |
| `portal.go` | Personal student endpoints, password changes, academic summary, filtered lists, fictional seed data |
| `db.go`, `schema.sql` | SQLite access, relationships, grades, WAM |
| `web/` | Shared interface and role-specific pages |
| `api_test.go`, `auth_test.go` | Automated backend tests |
| `summary.go`, `summary_test.go` | Typed academic summaries, a structural interface, consistent read snapshots and verification |
| `cmd/language-demo/`, `LANGUAGE_DEMO_WINDOWS.bat` | Controlled goroutine/channel, error and nullable-pointer demonstrations |
| `tools/browser-smoke.mjs` | Optional browser workflow checks using installed Chrome and Node.js |
| `docs/ARCHITECTURE.md` | Architecture, API access matrix and ER model |
| `docs/TESTING.md` | Test commands, results and manual checks |
| `docs/ER_Diagram.svg`, `docs/System_Architecture.svg` | Exportable diagrams |
| `docs/screenshots/` | Screenshots of the running application using fictional records |
| `docs/verification/` | Recorded test, static-check, benchmark and demonstration results |

## Git and GitHub

The included `.gitignore` excludes databases, executables, logs and local secrets. Review files before committing; no remote repository is configured by this package.

```powershell
git init
git add .
git commit -m "Implement Admin and Student record management"
git branch -M main
```

Create an empty repository in your own GitHub account. Then add the exact repository URL GitHub gives you as `origin` and push `main`. If integrating into an existing clone, preserve its `.git` directory and history instead of initializing a second repository.

## Storage and deployment scope

The Assignment 2 summary uses typed `StudentProfile`, `AcademicUnit` and `AcademicSummary` structs. `Store` implicitly satisfies the `academicReader` contract used by summary handlers. Profile and units are read in one transaction, and the WAM and counts are calculated from the returned units. Missing marks and WAM serialize as JSON null; a recorded zero remains a present mark. The Admin CRUD response format is unchanged.

Double-click `LANGUAGE_DEMO_WINDOWS.bat` to run the included controlled language examples without a Go installation. The intentionally recovered nil-pointer panic is confined to this separate executable; it is not web application logic.

The original project and its database remain separate. Do not point this version at the old Admin/General database: its account schema has no secure student ownership mapping. Keep backups of your database; stop the server before copying the database file so SQLite's WAL is fully checkpointed.

This is a local academic application. Sessions are held in memory, so restarting the server signs everyone out. The default listener is loopback. Institutional deployment would additionally require HTTPS configuration, persistent session management, audit and recovery procedures, institutional academic policies, and a deployment/security review.
