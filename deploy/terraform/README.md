# parserod — Terraform (AWS)

Provisions the parsero SaaS on AWS as a reusable module:

- **ECS Fargate** running two services from one image — a **web** tier
  (`ROLE=web`, behind the ALB) and a **worker** tier (`ROLE=worker`, processes
  the scan queue and runs the scheduler) — so the tiers scale independently.
- **RDS PostgreSQL** (durable source of truth) and **ElastiCache Redis**
  (queue / cache / throttle).
- **Application Load Balancer** (HTTP, or HTTPS when you pass `certificate_arn`).
- **Secrets Manager** for the `DATABASE_URL` / `REDIS_URL` connection strings,
  injected into the tasks; a generated DB password.
- **Application Auto Scaling** (CPU target tracking) on both tiers.

It is a *bring-your-own-network* module: pass an existing `vpc_id` plus public
and private subnet IDs.

## Usage

```hcl
module "parserod" {
  source = "github.com/zvdy/parsero-go//deploy/terraform"

  name               = "parserod"
  vpc_id             = "vpc-0123456789abcdef0"
  public_subnet_ids  = ["subnet-aaa", "subnet-bbb"]
  private_subnet_ids = ["subnet-ccc", "subnet-ddd"]

  # image           = "ghcr.io/zvdy/parserod:2.0.0"
  # certificate_arn = "arn:aws:acm:us-east-1:...:certificate/..."
}

output "url" {
  value = module.parserod.url
}
```

A runnable example lives in [`examples/basic`](examples/basic).

```sh
cd examples/basic
terraform init
terraform apply \
  -var vpc_id=vpc-... \
  -var 'public_subnet_ids=["subnet-a","subnet-b"]' \
  -var 'private_subnet_ids=["subnet-c","subnet-d"]'
```

## Auth

Like the rest of parsero, authentication is delegated to whatever sits in front
of the ALB (oauth2-proxy, an API gateway, a CDN with auth). It must inject the
trusted identity header (`identity_header`, default `X-Auth-Request-Email`).

## Worker autoscaling on queue depth

Both tiers autoscale on CPU out of the box. To scale the **worker** tier on the
actual scan backlog (the ECS equivalent of the Helm chart's KEDA scaler),
publish a custom CloudWatch metric for the asynq pending-queue length and attach
a target-tracking policy to `aws_appautoscaling_target.worker`. CPU tracking is
a reasonable default until then.

## Notes

- `skip_final_snapshot = true` and single-AZ RDS keep the demo cheap — harden
  (Multi-AZ, deletion protection, final snapshots, backups) before production.
- ElastiCache here runs without auth inside private subnets; enable AUTH/TLS and
  wire the token through Secrets Manager for production.
