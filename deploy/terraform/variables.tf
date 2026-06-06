variable "name" {
  description = "Name prefix for all resources."
  type        = string
  default     = "parserod"
}

variable "vpc_id" {
  description = "VPC to deploy into (bring-your-own network)."
  type        = string
}

variable "public_subnet_ids" {
  description = "Public subnets for the internet-facing ALB (>= 2 AZs)."
  type        = list(string)
}

variable "private_subnet_ids" {
  description = "Private subnets for ECS tasks, RDS, and ElastiCache (>= 2 AZs)."
  type        = list(string)
}

variable "image" {
  description = "Container image for parserod (runs both web and worker roles)."
  type        = string
  default     = "ghcr.io/zvdy/parserod:latest"
}

variable "container_cpu" {
  description = "Fargate CPU units per task."
  type        = number
  default     = 256
}

variable "container_memory" {
  description = "Fargate memory (MiB) per task."
  type        = number
  default     = 512
}

variable "web_desired_count" {
  description = "Baseline number of web tasks."
  type        = number
  default     = 2
}

variable "worker_desired_count" {
  description = "Baseline number of worker tasks."
  type        = number
  default     = 1
}

variable "web_max_count" {
  description = "Max web tasks for autoscaling."
  type        = number
  default     = 10
}

variable "worker_max_count" {
  description = "Max worker tasks for autoscaling."
  type        = number
  default     = 20
}

variable "db_instance_class" {
  description = "RDS instance class."
  type        = string
  default     = "db.t4g.micro"
}

variable "db_allocated_storage" {
  description = "RDS allocated storage (GiB)."
  type        = number
  default     = 20
}

variable "redis_node_type" {
  description = "ElastiCache node type."
  type        = string
  default     = "cache.t4g.micro"
}

variable "identity_header" {
  description = "Trusted identity header injected by the upstream auth proxy."
  type        = string
  default     = "X-Auth-Request-Email"
}

variable "certificate_arn" {
  description = "ACM certificate ARN for HTTPS on the ALB. When empty, the ALB serves HTTP on :80 (put an auth proxy/CDN in front in production)."
  type        = string
  default     = ""
}

variable "scheduler_enabled" {
  description = "Run the recurring-scan scheduler on the worker tier."
  type        = bool
  default     = true
}

variable "tags" {
  description = "Tags applied to all resources."
  type        = map(string)
  default     = {}
}
