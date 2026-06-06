output "alb_dns_name" {
  description = "Public DNS name of the load balancer."
  value       = aws_lb.this.dns_name
}

output "url" {
  description = "Base URL of the service."
  value       = "${var.certificate_arn == "" ? "http" : "https"}://${aws_lb.this.dns_name}"
}

output "ecs_cluster" {
  description = "ECS cluster name."
  value       = aws_ecs_cluster.this.name
}

output "rds_address" {
  description = "RDS endpoint hostname."
  value       = aws_db_instance.this.address
}

output "redis_address" {
  description = "ElastiCache primary endpoint."
  value       = aws_elasticache_cluster.this.cache_nodes[0].address
}

output "database_url_secret_arn" {
  description = "Secrets Manager ARN holding DATABASE_URL."
  value       = aws_secretsmanager_secret.database_url.arn
}
