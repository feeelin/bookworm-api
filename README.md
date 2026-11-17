# Bookworm API

Bookworm API is a small Go REST API for managing a book collection with Gin and SQLite. This repository contains the CI/CD automation built throughout the **Modern CI/CD** course, from merge request checks to production deployment and rollback.

## What the course automates

The [GitLab CI/CD pipeline](.gitlab-ci.yml) demonstrates how a change moves from review to release.

- **Review.** Validate commit messages with Commitizen, run golangci-lint and PVS-Studio, scan dependencies with Trivy, and detect secrets with Gitleaks.
- **Test.** Run unit tests with race detection and coverage, then run API integration tests. GitLab collects JUnit reports for both suites.
- **Package.** Check that the Docker image builds in merge requests. On the default branch and for Git tags, publish an image tagged with the short commit SHA or Git tag and embed build metadata in the binary.
- **Deploy.** Roll out the image to [staging](deploy/staging/docker-compose.yml). For tagged releases, [deploy the same image to production](deploy/production/deploy.sh) using a blue-green rollout. The script checks health and version before switching traffic. A [manual rollback](deploy/production/rollback.sh) switches traffic back to the previous instance.

## Run locally

With Go 1.26.4 and a C compiler for SQLite installed, run:

```sh
go run .
```

Open [Swagger UI](http://localhost:8080/swagger/index.html) to explore the `/books` endpoints. Use `/health` and `/version` to check the running service.
