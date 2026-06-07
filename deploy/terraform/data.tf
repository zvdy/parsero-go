# Managed Postgres (durable source of truth) and Redis (queue/cache/throttle).

resource "aws_db_subnet_group" "this" {
  name_prefix = "${var.name}-"
  subnet_ids  = var.private_subnet_ids
  tags        = local.tags
}

resource "aws_db_instance" "this" {
  identifier_prefix      = "${var.name}-"
  engine                 = "postgres"
  engine_version         = "16"
  instance_class         = var.db_instance_class
  allocated_storage      = var.db_allocated_storage
  storage_encrypted      = true
  db_name                = "parsero"
  username               = "parsero"
  password               = random_password.db.result
  db_subnet_group_name   = aws_db_subnet_group.this.name
  vpc_security_group_ids = [aws_security_group.data.id]
  multi_az               = false
  skip_final_snapshot    = true
  apply_immediately      = true
  tags                   = local.tags
}

resource "aws_elasticache_subnet_group" "this" {
  name       = "${var.name}-redis"
  subnet_ids = var.private_subnet_ids
  tags       = local.tags
}

resource "aws_elasticache_cluster" "this" {
  cluster_id           = "${var.name}-redis"
  engine               = "redis"
  engine_version       = "7.1"
  node_type            = var.redis_node_type
  num_cache_nodes      = 1
  parameter_group_name = "default.redis7"
  port                 = 6379
  subnet_group_name    = aws_elasticache_subnet_group.this.name
  security_group_ids   = [aws_security_group.data.id]
  tags                 = local.tags
}
