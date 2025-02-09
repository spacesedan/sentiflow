#!/bin/bash
set -e

# Update the system and install required tools
yum update -y
yum install -y wget curl tar

# Hardcoded Go version
echo "Installing Go version: 1.21.0" >>/var/log/user-data.log

# Download and install Go
wget "https://go.dev/dl/go1.21.0.linux-amd64.tar.gz" -O /tmp/go.tar.gz
rm -rf /usr/local/go
tar -C /usr/local -xzf /tmp/go.tar.gz
rm -f /tmp/go.tar.gz

# Set up Go environment variables
echo "export GOPATH=/home/ec2-user/go" >>/home/ec2-user/.bash_profile
echo "export PATH=\$PATH:/usr/local/go/bin:\$GOPATH/bin" >>/home/ec2-user/.bash_profile

# Apply profile changes
source /home/ec2-user/.bash_profile

# Create the GOPATH directories
mkdir -p /home/ec2-user/go/{bin,src,pkg}

# Verify installation
/usr/local/go/bin/go version >>/var/log/user-data.log
