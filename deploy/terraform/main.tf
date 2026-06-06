locals {
  tags = merge({ "app" = var.name, "managed-by" = "terraform" }, var.tags)

  # Connection strings consumed by parserod. SSL on for RDS; ElastiCache here is
  # provisioned without auth inside the private subnets.
  database_url = "postgres://parsero:${random_password.db.result}@${aws_db_instance.this.address}:${aws_db_instance.this.port}/parsero?sslmode=require"
  redis_url    = "redis://${aws_elasticache_cluster.this.cache_nodes[0].address}:6379/0"
}

# ---------------------------------------------------------------------------
# Security groups
# ---------------------------------------------------------------------------
resource "aws_security_group" "alb" {
  name_prefix = "${var.name}-alb-"
  description = "parserod ALB"
  vpc_id      = var.vpc_id
  tags        = local.tags

  ingress {
    description = "web"
    from_port   = var.certificate_arn == "" ? 80 : 443
    to_port     = var.certificate_arn == "" ? 80 : 443
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }
  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }
}

resource "aws_security_group" "tasks" {
  name_prefix = "${var.name}-tasks-"
  description = "parserod ECS tasks"
  vpc_id      = var.vpc_id
  tags        = local.tags

  ingress {
    description     = "from ALB"
    from_port       = 8080
    to_port         = 8080
    protocol        = "tcp"
    security_groups = [aws_security_group.alb.id]
  }
  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }
}

resource "aws_security_group" "data" {
  name_prefix = "${var.name}-data-"
  description = "parserod RDS + Redis"
  vpc_id      = var.vpc_id
  tags        = local.tags

  ingress {
    description     = "postgres"
    from_port       = 5432
    to_port         = 5432
    protocol        = "tcp"
    security_groups = [aws_security_group.tasks.id]
  }
  ingress {
    description     = "redis"
    from_port       = 6379
    to_port         = 6379
    protocol        = "tcp"
    security_groups = [aws_security_group.tasks.id]
  }
  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }
}

# ---------------------------------------------------------------------------
# Secrets (connection strings injected into the tasks)
# ---------------------------------------------------------------------------
resource "random_password" "db" {
  length  = 24
  special = false
}

resource "aws_secretsmanager_secret" "database_url" {
  name_prefix = "${var.name}/database-url-"
  tags        = local.tags
}

resource "aws_secretsmanager_secret_version" "database_url" {
  secret_id     = aws_secretsmanager_secret.database_url.id
  secret_string = local.database_url
}

resource "aws_secretsmanager_secret" "redis_url" {
  name_prefix = "${var.name}/redis-url-"
  tags        = local.tags
}

resource "aws_secretsmanager_secret_version" "redis_url" {
  secret_id     = aws_secretsmanager_secret.redis_url.id
  secret_string = local.redis_url
}
