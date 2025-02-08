resource "aws_vpc" "this" {
  cidr_block = var.vpc_cidr
}

resource "aws_subnet" "public_subnet" {
  vpc_id                  = aws_vpc.this.id
  cidr_block              = var.public_subnet_cidr
  map_public_ip_on_launch = true
  availability_zone_id    = var.availability_zone
}

resource "aws_subnet" "private_subnet" {
  vpc_id               = aws_vpc.this.id
  cidr_block           = var.private_subnet_cidr
  availability_zone_id = var.availability_zone
}

# Internet Gateway for public subnets
resource "aws_internet_gateway" "this" {
  vpc_id = aws_vpc.this.id
}

# Public route table
resource "aws_route_table" "public_rt" {
  vpc_id = aws_vpc.this.id
}

resource "aws_route" "public_route" {
  route_table_id         = aws_route_table.public_rt.id
  destination_cidr_block = "0.0.0.0/0"
  gateway_id             = aws_internet_gateway.this.id
}

resource "aws_route_table_association" "publice_subnet_assoc" {
  subnet_id      = aws_subnet.public_subnet.id
  route_table_id = aws_route_table.public_rt.id
}
