---
name: containerization
description: Build, configure, and operate containerized workloads with Docker and Kubernetes. Use when creating Dockerfiles, writing Kubernetes manifests, or debugging container runtime issues.
license: Apache-2.0
compatibility: Works with coding agents that support Agent Skills.
metadata:
  agentpacks.version: "0.1.0"
---

# Containerization

Containers are unit boundaries. Build them small, run them immutably, and treat them as cattle — not pets.

## Dockerfile Best Practices

- Use pinned, minimal base images: `node:22-alpine` not `node:latest`. Re-pin regularly on a schedule.
- Order layers from least-to-most volatile: `FROM` → `RUN apt-get` → `COPY package.json` → `RUN npm install` → `COPY . .`. Build cache is invalidated at the first changed layer.
- Combine `RUN` steps that share a logical operation into one to minimize layer count and leak of intermediate files.
- Use multi-stage builds: compile/install in one stage, copy only artifacts to the final stage. Excludes dev tools, source code, and build caches from the image.
- Run the final process as a non-root user: `RUN adduser --disabled-password app && USER app`.
- Use `COPY --chown=app:app` instead of a separate `RUN chown`.
- Set `ENTRYPOINT` to the process, `CMD` to its default arguments — so callers can override args without replacing the binary.

## Image Hygiene

- Keep images under 200 MB where feasible. Audit with `docker image history` and `dive`.
- `.dockerignore` must exclude: `node_modules`, `.git`, build artifacts, `.env` files, and test directories.
- Never bake secrets into image layers. Use build secrets (`--secret id=...`) or mount at runtime.
- Tag images with the git commit SHA in CI: `myapp:$GIT_SHA`. `latest` is a debugging alias, not a deployment target.
- Scan images with `docker scout` or Trivy before pushing to a registry.

## Kubernetes Manifests

- Set `resources.requests` and `resources.limits` on every container. Missing requests prevent the scheduler from placing pods correctly.
- Use `livenessProbe` and `readinessProbe` on all long-running containers. `readinessProbe` gates traffic; `livenessProbe` restarts stuck processes.
- Set `terminationGracePeriodSeconds` high enough for in-flight requests to drain (`30s` is often too low for HTTP services).
- Use `PodDisruptionBudget` for stateful sets and critical services to prevent zero-availability during rolling updates.
- Apply labels consistently: `app.kubernetes.io/name`, `app.kubernetes.io/version`, `app.kubernetes.io/component`.
- Never use `hostNetwork: true` or `privileged: true` unless the workload genuinely requires it.

## ConfigMaps and Secrets

- Mount secrets as files, not environment variables — env vars appear in crash dumps and `ps` output.
- Rotate secrets by updating the Secret object and rolling the Deployment — pods do not automatically reload mounted secrets.
- Use `SecretProviderClass` (CSI driver) or an operator to sync secrets from a vault into Kubernetes rather than storing them in etcd.

## Checklist

- [ ] Dockerfile uses pinned base image and multi-stage build.
- [ ] Image runs as non-root with read-only root filesystem where possible.
- [ ] `.dockerignore` excludes secrets, `.git`, and build artifacts.
- [ ] Image scanned for CVEs before pushing.
- [ ] All pods have resource requests/limits, liveness, and readiness probes.
- [ ] Secrets mounted as files, not env vars.
- [ ] `PodDisruptionBudget` in place for critical workloads.
