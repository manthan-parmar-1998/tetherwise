# Lab

The lab is a local, synthetic AWS account. Later lessons will recreate a shared-OpenSearch overload here, without a cloud account and without real names.

## Start the emulator

From this directory:

```sh
docker compose up
```

Floci listens on port 4566. The image is `floci/floci:latest`, a floating tag for this local lab. Pin a digest when you want the lab to stay on one build.

The Docker socket is mounted because OpenSearch, RDS, and MSK in Floci start as containers. That socket can control the local Docker daemon. Use this compose file only on a machine you trust, and only with the dummy credentials below.

`container_name` is `floci` so the Floci CLI can find the container.

Podman works if you point the volume at the Podman socket. The path in the compose file is the Docker socket.

## Point your tools at it

With the [Floci CLI](https://floci.io/floci/getting-started/installation/) installed:

```sh
eval $(floci env)
```

That exports `AWS_ENDPOINT_URL`, `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, and `AWS_DEFAULT_REGION`. The credentials are dummy values for the emulator.

Or export them yourself:

```sh
export AWS_ENDPOINT_URL=http://localhost:4566
export AWS_ACCESS_KEY_ID=test
export AWS_SECRET_ACCESS_KEY=test
export AWS_DEFAULT_REGION=us-east-1
```

Check that the CLI is talking to the emulator before you do anything else:

```sh
aws sts get-caller-identity
```

The answer should come from the emulator. If `AWS_ENDPOINT_URL` is unset, the AWS CLI talks to real AWS. Stop and fix the environment if that happens.

## What the lab will become

[`scenarios/opensearch-shared-overload/`](scenarios/opensearch-shared-overload/README.md) is the story of a synthetic shared-OpenSearch overload. The Terraform, Kubernetes config, edges file, and CloudWatch samples are not in this directory yet. Manthan will add them while writing the source and metrics packages. Until then, `docker compose up` only starts the emulator.

## CI, later

Floci is the local lab. Contract tests in CI, when they are added, run against the official [AWS Emulator](https://aws.amazon.com/blogs/aws/test-your-applications-against-aws-locally-with-aws-emulator/): start it in the job, point the SDK at its local endpoint, and assert `graph` and `check` on synthetic resources. Those tests are not in this scaffold.

Neither Floci nor AWS Emulator evaluates IAM the way AWS does. A call AWS would deny can succeed in either one. The read-only safety gate in `internal/safety` still needs a final test against a throwaway real AWS account: no production data, credentials that can be discarded, and no standing CI job pointed at that account.
