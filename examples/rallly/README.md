# Rallly — an example instance definition

The first instance definition an author will see, for
[Rallly](https://github.com/lukevella/rallly): a pnpm/turbo monorepo with a
Next.js app, Prisma, and a docker-compose dev stack — structurally the shape
Rolling Start's real targets take. It is also the target M1's exit criterion
is measured against (`docs/plans/m1-rolling-doctor.md`).

## Use it

```sh
git clone https://github.com/lukevella/rallly
cp -r examples/rallly/.rollingstart rallly/
cd rallly && git add .rollingstart && git commit -m "Rolling Start instance definition"
rolling doctor
```

Commit the directory: an untracked `.rollingstart/` is exactly what the
working-tree probe reports, and it is right to.

## What the commands are

| key | command | why this one |
|---|---|---|
| `build` | `pnpm build` | Rallly's own build of the web app |
| `typecheck` | `pnpm type-check` | `turbo type-check` across the workspace |
| `test` | `pnpm test:unit` | unit tests need no infrastructure; the integration suite needs the stack, which makes it an operation (M2), not a health check |
| `lint` | `pnpm check` | biome, as Rallly runs it |

## What green requires

Rallly declares `pnpm@10.28.0` (`packageManager`) and node 24 (`engines`).
The second section of the report goes green after `pnpm install` — and for
anything that reaches the database, after Rallly's own `pnpm docker:up`.
Rolling Start never brings the stack up; it reports what the toolchain says
about the working copy, stack or no stack. For scale: on the validation run
`build` took about 35 seconds, the first `type-check` 25 and the first
`test:unit` 8 (sub-second once turbo's cache is warm), `check` 2 — all well
inside doctor's default five-minute bound.

## Reproducing the validation

The transcripts on the validation PR come from this recipe. Nothing is
installed on the host: one throwaway directory, the toolchain in the
official `node:24` image, Rallly's stack on the host daemon via Rallly's
own compose file. The host keeps only the pulled images.

```sh
W=$(mktemp -d /tmp/rallly-validation.XXXX)
git clone --depth 1 https://github.com/lukevella/rallly "$W/rallly"
cp -r examples/rallly/.rollingstart "$W/rallly/"
git -C "$W/rallly" add .rollingstart && git -C "$W/rallly" commit -qm "instance definition"
go build -o "$W/rolling" ./cmd/rolling

# Broken environment: node:24 ships corepack but pnpm is not on PATH —
# the "fresh clone, no pnpm" state, on any machine.
docker run --rm --user "$(id -u):$(id -g)" -e HOME=/tmp \
  -v "$W/rallly:/work" -v "$W/rolling:/usr/local/bin/rolling:ro" -w /work \
  node:24 rolling doctor

# Healthy environment — Rallly's own CONTRIBUTING.md steps, inside the
# container: pnpm enabled into the container's /tmp (and its store kept
# out of the checkout, or the tree is dirty), env from the samples with
# the one required value filled in, prisma generate, then the database
# reset and seeded against the stack. The stack comes up by Rallly's
# compose file; host networking makes its localhost ports reachable
# (Linux — on macOS point the .env files at host.docker.internal).
docker compose -f "$W/rallly/docker-compose.dev.yml" up -d --wait
docker run --rm --network host --user "$(id -u):$(id -g)" -e HOME=/tmp \
  -v "$W/rallly:/work" -v "$W/rolling:/usr/local/bin/rolling:ro" -w /work \
  node:24 sh -c '
    mkdir -p /tmp/bin && COREPACK_HOME=/tmp/corepack corepack enable --install-directory /tmp/bin
    export PATH=/tmp/bin:$PATH npm_config_store_dir=/tmp/pnpm-store
    pnpm install
    cp apps/web/.env.sample apps/web/.env && cp packages/database/.env.sample packages/database/.env
    sed -i "s/^SECRET_PASSWORD=$/SECRET_PASSWORD=$(head -c 24 /dev/urandom | base64)/" apps/web/.env
    pnpm db:generate && pnpm db:reset --force && pnpm db:seed
    rolling doctor'

# Teardown
docker compose -f "$W/rallly/docker-compose.dev.yml" down --volumes --remove-orphans
rm -rf "$W"
```

Three lines in that recipe were learned the hard way, and each is a true
finding doctor made about the environment rather than a bug in doctor:
`--user` and `HOME`, or git inside the container sees a root process touching
a repository owned by your user and the first probe goes red with "dubious
ownership"; `npm_config_store_dir`, or pnpm — with `HOME` on a different
filesystem from the checkout — drops its store *inside* the project, the
working tree goes red, and biome lints two thousand JSON files; and the
`SECRET_PASSWORD` line, without which Rallly's build fails env validation
with `Invalid input` — the sample ships it empty, and CONTRIBUTING's "fill in
the required values" means exactly that.
