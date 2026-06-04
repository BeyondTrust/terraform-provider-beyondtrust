# Contributing to the BeyondTrust Workload Credentials Terraform Provider

Thank you for your interest in contributing to our project!

Here is some information on how to get started and where to ask for help.

## Getting Started

The BeyondTrust Workload Credentials Terraform Provider is a Terraform integration for BeyondTrust Workload Credentials. It enables infrastructure-as-code management of secrets, folders, AWS integrations, and dynamic credential templates.

### Development Resources

- **[DEVELOPMENT.md](./docs/development/DEVELOPMENT.md)** - Local development setup and workflow
- **[TESTING.md](./docs/development/TESTING.md)** - Running unit and acceptance tests
- **[QUICKSTART.md](./docs/guides/quickstart.md)** - Quick start guide for using the provider
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
6. **Verify signing**: Ensure your commits are signed with `git log -1 --show-signature`

### Pull Request Process

**Before starting:** Ensure you have configured commit signing (see "Branch Protection and Signed Commits" below)

1. Fork the repository and create your branch from `main`
2. Make your changes following the existing code style
3. Add or update tests as necessary
4. Update documentation if you changed functionality
5. Ensure all tests and checks pass
6. Submit a pull request with signed commits and a clear description of the changes

**Note:** If you need to rebase or amend commits during review, ensure you preserve signatures using the `-S` flag or rebase with `--exec 'git commit --amend --no-edit -S'`.

### Branch Protection and Signed Commits

BeyondTrust requires signed commits across all public repositories for security, authenticity, and compliance. Signing commits ensures that contributions come from verified sources and helps maintain the integrity of our codebase. This aligns with industry best practices and BeyondTrust's security standards across all repositories.

All commits must be cryptographically signed using either GPG or SSH keys. Both methods are equally supported—choose the one that best fits your workflow.

#### Option A: GPG Signing

**1. Generate a GPG key (if you don't have one)**

```bash
# Generate a new GPG key (choose RSA 4096-bit)
gpg --full-generate-key

# List your GPG keys to get the key ID
gpg --list-secret-keys --keyid-format=long

# Example output:
# sec   rsa4096/ABC123DEF456 2024-01-01 [SC]
#       Your key ID is: ABC123DEF456
```

**2. Export your public key and add it to GitHub**

```bash
# Export your public key (replace KEY_ID with your actual key ID)
gpg --armor --export KEY_ID

# Copy the output and add it to GitHub at:
# https://github.com/settings/gpg/new
```

**3. Configure Git to sign commits with your GPG key**

```bash
# Set your GPG signing key
git config --global user.signingkey KEY_ID

# Enable automatic commit signing
git config --global commit.gpgsign true

# Enable automatic tag signing
git config --global tag.gpgsign true

# Ensure your Git email matches your GitHub account email
git config --global user.email "your-github-email@example.com"
```

**4. Configure GPG TTY (required for terminal usage)**

Add to your shell profile (`~/.bashrc`, `~/.zshrc`, etc.):

```bash
export GPG_TTY=$(tty)
```

Then reload your shell or run `source ~/.bashrc` (or `~/.zshrc`).

#### Option B: SSH Signing

**1. Generate an SSH key (if you don't have one)**

```bash
# Generate a new SSH key (ed25519 is recommended)
ssh-keygen -t ed25519 -C "your-github-email@example.com"

# Or use an existing SSH key (typically ~/.ssh/id_ed25519.pub or ~/.ssh/id_rsa.pub)
```

**2. Configure Git for SSH signing**

```bash
# Tell Git to use SSH for signing
git config --global gpg.format ssh

# Set your SSH signing key (use the PUBLIC key path)
git config --global user.signingkey ~/.ssh/id_ed25519.pub

# Enable automatic commit signing
git config --global commit.gpgsign true

# Ensure your Git email matches your GitHub account email
git config --global user.email "your-github-email@example.com"
```

**3. Add your SSH key to GitHub as a Signing Key**

**CRITICAL:** You must add your SSH public key to GitHub as a **Signing Key**, not just an authentication key.

1. Copy your SSH public key:
   ```bash
   cat ~/.ssh/id_ed25519.pub
   ```

2. Go to: <https://github.com/settings/ssh/new>
3. Select **Key type: Signing Key** (not "Authentication Key")
4. Paste your public key
5. Click "Add SSH key"

#### Verification

After configuring either GPG or SSH signing, verify your setup:

```bash
# Create a test commit
git commit --allow-empty -m "test: verify commit signing"

# Verify the commit is signed
git log -1 --show-signature
```

**Expected output for GPG:**
```text
gpg: Signature made [date]
gpg: Good signature from "Your Name <email@example.com>"
```

**Expected output for SSH:**
```text
Good "git" signature for your-github-email@example.com with ED25519 key SHA256:...
```

#### Integration with Repository Workflow

Commit signing works seamlessly with this repository's development tools:

```bash
# Commits created via pre-commit hooks are automatically signed
make pre-commit-quick

# Git hooks installed via make also respect signing configuration
make install-git-hooks

# When rebasing, preserve signatures with:
git rebase --exec 'git commit --amend --no-edit -S' main

# When amending commits, the -S flag ensures signing:
git commit --amend -S
```

#### Troubleshooting

**GPG: "gpg: signing failed: Inappropriate ioctl for device"**
```bash
# Add to your shell profile (~/.bashrc, ~/.zshrc, etc.)
export GPG_TTY=$(tty)

# Then reload your shell
source ~/.bashrc  # or ~/.zshrc
```

**GPG: "error: gpg failed to sign the data"**
```bash
# Verify your signing key is configured
git config --get user.signingkey

# Verify GPG can access your key
gpg --list-secret-keys --keyid-format=long

# Test GPG signing directly
echo "test" | gpg --clearsign
```

**SSH: Commits still showing as unsigned**
```bash
# Verify signing is enabled
git config --get commit.gpgsign

# Verify GPG format is set to SSH
git config --get gpg.format

# Verify your signing key path
git config --get user.signingkey

# Ensure the key was added to GitHub as "Signing Key" (not just authentication)
# Visit: https://github.com/settings/keys
```

**SSH: "error: gpg.ssh.allowedSignersFile needs to be configured"**

This warning can be ignored for GitHub workflows. GitHub validates SSH signatures using keys registered in your account.

**General: Need to verify commit signatures locally**
```bash
# For SSH, optionally configure allowed signers file
git config --global gpg.ssh.allowedSignersFile ~/.ssh/allowed_signers

# Create the allowed signers file
echo "$(git config --get user.email) $(cat ~/.ssh/id_ed25519.pub)" > ~/.ssh/allowed_signers
```

**Still having issues?**
- Contact [BeyondTrust Support](https://www.beyondtrust.com/support)
- Review GitHub's documentation:
  - [GPG signing](https://docs.github.com/en/authentication/managing-commit-signature-verification/signing-commits)
  - [SSH signing](https://docs.github.com/en/authentication/managing-commit-signature-verification/about-commit-signature-verification#ssh-commit-signature-verification)

### Commit Message Guidelines

- Use clear, descriptive commit messages
- Follow [Conventional Commits](https://www.conventionalcommits.org/) format:
  - `feat:` for new features
  - `fix:` for bug fixes
  - `docs:` for documentation changes
  - `test:` for test additions/changes
  - `refactor:` for code refactoring
  - `chore:` for maintenance tasks

Example: `git commit -S -m "feat: add support for OIDC trust policies"`

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
