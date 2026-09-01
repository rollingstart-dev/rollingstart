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
about the working copy, stack or no stack.

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

# Healthy environment: pnpm enabled into the container's own /tmp, the
# stack up by Rallly's compose file, host networking so localhost ports
# resolve (Linux; on macOS point Rallly's .env at host.docker.internal).
docker compose -f "$W/rallly/docker-compose.dev.yml" up -d --wait
docker run --rm --network host --user "$(id -u):$(id -g)" -e HOME=/tmp \
  -v "$W/rallly:/work" -v "$W/rolling:/usr/local/bin/rolling:ro" -w /work \
  node:24 sh -c 'COREPACK_HOME=/tmp/corepack corepack enable --install-directory /tmp/bin \
    && export PATH=/tmp/bin:$PATH && pnpm install && rolling doctor'

# Teardown
docker compose -f "$W/rallly/docker-compose.dev.yml" down --volumes --remove-orphans
rm -rf "$W"
```

The `--user` and `HOME` flags are not decoration: without them git inside
the container sees a root process touching a repository owned by your user
and the first probe goes red with "dubious ownership" — a true finding, but
not the one being tested.
