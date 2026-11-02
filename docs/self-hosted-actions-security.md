# Secure self-hosted GitHub Actions

This is a design for the existing Ryzen 5 2600 / 48 GB always-on server.
It does not change any GitHub Actions workflow or runner registration.

## Trust boundary

Never register a persistent runner directly to a public StashBooru pull-request
workflow: fork authors can submit executable workflow changes. Labels alone,
PR approval alone, and an unprivileged Linux user alone are not containment.

Preferred architecture:

1. Run a clean, disposable Linux VM (KVM/libvirt or Proxmox) on the server,
   around 4 vCPU, 12-16 GiB RAM and 50 GiB SSD workspace, one job at a time.
2. Isolate the VM from the host, LAN, router, Home Assistant, private storage,
   SSH agent and Docker socket. Allow only required outbound GitHub/package
   endpoints. Do not mount host media directories or secrets.
3. Create a separate PRIVATE GitHub repository such as
   Dusky-dev/ci-runner-hub. Register the runner at the hub repository level,
   not as a repository runner in public StashBooru.
4. Only owner-maintained manual-dispatch workflows in that private hub can
   schedule tests. Validate a full 40-digit commit SHA and use that exact
   revision; never evaluate user input as shell code.
5. In the disposable VM, follow GitHub's verified runner download procedure.
   Register with ./config.sh --ephemeral using a fresh, short-lived token
   supplied outside the VM base image, then start ./run.sh.
6. Destroy the VM, all caches and its registration-token state after each job,
   restore the clean base image, and provision a fresh runner for the next job.
   Forward runner logs to protected storage outside the guest if needed.

The private hub can test multiple GitHub repositories by checking out each
repository from owner-approved commits. For private worm-ultimate, use a
separate narrowly scoped read-only credential for checkout and never pass
it to PR-controlled code. Do not register the same runner directory to
multiple repositories.

## Concrete setup sketch

From the new runner installation directory INSIDE the disposable VM, as a
non-root user, after obtaining the private hub's one-time registration token:

    ./config.sh --url https://github.com/Dusky-dev/ci-runner-hub \
      --token "$FRESH_REGISTRATION_TOKEN" \
      --name stashbooru-ephemeral \
      --labels stashbooru-isolated --ephemeral --unattended
    ./run.sh

This is a manual smoke test, not a daemon. The VM must be reset after the
runner exits; --ephemeral alone does not wipe the guest.

In the PRIVATE hub workflow, use workflow_dispatch only; constrain
github.actor to Dusky-dev, set permissions to the minimum, validate inputs
against a complete Git commit hash, and use the following job configuration:

    runs-on: [self-hosted, linux, x64, stashbooru-isolated]
    timeout-minutes: 60

Checkout StashBooru at the verified SHA with persist-credentials: false,
set up Go, and run make test. Other tests can be added after the smoke test.

Do not add these labels to the public PR workflows. In particular, do not
change the existing public booru-browser/build/lint workflows' runs-on
values yet, and never execute fork code using pull_request_target with
privileged credentials.

## Agent development

The GitHub connection remains available for source inspection, branches,
PRs, code review and CI status. It does not need shell access to the runner.
The owner can manually run approved commits in the private CI hub while the
existing public PR checks remain on GitHub-hosted runners. Integrating
trusted check results back into PRs later requires a narrowly scoped
GitHub App or an equally reviewed mechanism; do not expose a dispatch
secret to arbitrary PRs.

A personal-account repository runner belongs to one repository. If the
repos are ever moved into a GitHub organization, runner groups can restrict
both selected repositories and selected workflows; transferring repositories
is not necessary for this private-hub arrangement.

## Official references

- https://docs.github.com/en/actions/reference/security/secure-use
- https://docs.github.com/en/actions/reference/runners/self-hosted-runners
- https://docs.github.com/en/actions/how-tos/manage-runners/self-hosted-runners/manage-access
