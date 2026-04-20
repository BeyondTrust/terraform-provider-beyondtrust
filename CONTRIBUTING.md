# Contributing to the BeyondTrust Workload Credentials Terraform Provider

Thank you for your interest in contributing to our project!

Here is some information on how to get started and where to ask for help.

## Getting Started

The BeyondTrust Workload Credentials Terraform Provider is a Terraform integration for BeyondTrust Workload Credentials. It enables infrastructure-as-code management of secrets, folders, AWS integrations, and dynamic credential templates.

### Development Resources

- **[DEVELOPMENT.md](DEVELOPMENT.md)** - Local development setup and workflow
- **[TESTING.md](TESTING.md)** - Running unit and acceptance tests
- **[QUICKSTART.md](QUICKSTART.md)** - Quick start guide for using the provider
- **[README.md](README.md)** - Provider documentation and usage examples

## How can I Contribute?

### Reporting Bugs

Bugs should be submitted through [BeyondTrust Support](https://www.beyondtrust.com/support). Any bugs should be submitted against _BeyondTrust Workload Credentials Support_. Our support team will ensure the escalation is raised to the proper team internally.

If the bug is a security vulnerability, instead please refer to our [Security Policy](SECURITY.md) or the [responsible disclosure section of our security policy](https://www.beyondtrust.com/security#disclosure).

### Feature Requests

Feature requests should also be submitted through [BeyondTrust Support](https://www.beyondtrust.com/support), also against _BeyondTrust Workload Credentials Support_. Submitting through our support organization will ensure the request gets sent to the proper Product Management team for consideration.

### Suggesting a Code Change

#### **Did you write a patch that fixes a bug?**

- Submit through [BeyondTrust Support](https://www.beyondtrust.com/support).
- Ensure you describe clearly both the problem and the solution.
- Include steps to reproduce the issue.
- Provide details about your environment (Terraform version, provider version, OS).

#### **Do you have ideas for a new feature or change an existing one?**

- Consider submitting a feature request through [BeyondTrust Support](https://www.beyondtrust.com/support) to ensure that your proposed changes do not conflict with new features that are already planned or in development.
- For significant changes, open a discussion first to get feedback on the approach.

## Development Guidelines

### Before Submitting a Pull Request

1. **Run tests**: Ensure all unit tests pass with `make test-unit`
2. **Format code**: Run `make fmt` to format Go and Terraform code
3. **Lint**: Run `make lint` to catch common issues
4. **Generate docs**: Run `make generate` if you changed resource schemas
5. **Pre-commit checks**: Run `make pre-commit` for a full validation

### Pull Request Process

1. Fork the repository and create your branch from `main`
2. Make your changes following the existing code style
3. Add or update tests as necessary
4. Update documentation if you changed functionality
5. Ensure all tests and checks pass
6. Submit a pull request with a clear description of the changes

### Commit Message Guidelines

- Use clear, descriptive commit messages
- Follow [Conventional Commits](https://www.conventionalcommits.org/) format:
  - `feat:` for new features
  - `fix:` for bug fixes
  - `docs:` for documentation changes
  - `test:` for test additions/changes
  - `refactor:` for code refactoring
  - `chore:` for maintenance tasks

Example: `feat: add support for OIDC trust policies`

## Code of Conduct

We are committed to providing a welcoming and inclusive environment for all contributors. Please treat all community members with respect and professionalism.

## Questions?

If you have questions about contributing, please:
- Check existing documentation in the repository
- Contact [BeyondTrust Support](https://www.beyondtrust.com/support)
- Review closed issues and pull requests for similar discussions

## License

By contributing to this project, you agree that your contributions will be licensed under the same license as the project (see [LICENSE](LICENSE) file).

---

Thank you for contributing to the BeyondTrust Workload Credentials Terraform Provider!
