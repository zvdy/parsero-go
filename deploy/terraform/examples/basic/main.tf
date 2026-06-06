terraform {
  required_version = ">= 1.5"
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = ">= 5.0"
    }
  }
}

provider "aws" {
  region = var.region
}

module "parserod" {
  source = "../.."

  name               = "parserod"
  vpc_id             = var.vpc_id
  public_subnet_ids  = var.public_subnet_ids
  private_subnet_ids = var.private_subnet_ids

  # Optionally pin a specific image and enable HTTPS:
  # image           = "ghcr.io/zvdy/parserod:2.0.0"
  # certificate_arn = "arn:aws:acm:...:certificate/..."

  tags = {
    env = "demo"
  }
}

output "url" {
  value = module.parserod.url
}
