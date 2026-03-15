#!/usr/bin/env bash
# ============================================================
# FILE: scripts/aws_ec2_setup.sh
# TYPE: Automation Script — EC2 Bootstrapping
#
# WHAT IS THIS?
# A bash script to run ONCE on a brand new Ubuntu EC2 instance.
# It installs Docker, Docker Compose, Python pip, and sets up
# the correct user permissions so you don't have to type 'sudo'
# before every docker command.
#
# WHY BASH FOR THIS?
# Bootstrapping operating systems is what Bash does best. 
# In enterprise FAANG environments, this script would be pasted 
# into the "AWS User Data" field so it runs automatically the 
# second the server turns on.
# ============================================================

set -euo pipefail

# ANSI color codes for pretty output
GREEN='\033[0;32m'
CYAN='\033[0;36m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo -e "${CYAN}==============================================${NC}"
echo -e "${CYAN}   🚀 Starting AWS EC2 Installation Script    ${NC}"
echo -e "${CYAN}==============================================${NC}"

# 1. Update package lists
echo -e "\n${YELLOW}1. Updating Ubuntu packages...${NC}"
sudo apt-get update -y
sudo apt-get upgrade -y

# 2. Install prerequisites
echo -e "\n${YELLOW}2. Installing prerequisites (curl, git, python3-pip)...${NC}"
sudo apt-get install -y ca-certificates curl gnupg lsb-release git python3-pip python3-venv

# 3. Add Docker's official GPG key
echo -e "\n${YELLOW}3. Fetching Docker GPG keys...${NC}"
sudo mkdir -p /etc/apt/keyrings
curl -fsSL https://download.docker.com/linux/ubuntu/gpg | sudo gpg --dearmor -y -o /etc/apt/keyrings/docker.gpg

# 4. Set up the Docker repository
echo -e "\n${YELLOW}4. Adding Docker repository to apt...${NC}"
echo \
  "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/ubuntu \
  $(lsb_release -cs) stable" | sudo tee /etc/apt/sources.list.d/docker.list > /dev/null

# 5. Install Docker Engine and Docker Compose
echo -e "\n${YELLOW}5. Installing Docker Engine and Docker Compose...${NC}"
sudo apt-get update -y
sudo apt-get install -y docker-ce docker-ce-cli containerd.io docker-compose-plugin

# 6. Post-installation steps for Docker (so we don't need 'sudo docker')
echo -e "\n${YELLOW}6. Configuring Docker user permissions...${NC}"
# Create the docker group if it doesn't exist
sudo groupadd docker || true
# Add the current user (usually 'ubuntu' on EC2) to the docker group
sudo usermod -aG docker $USER

# 7. Install Python psycopg2 so the seed scripts work directly on the host
echo -e "\n${YELLOW}7. Installing Python dependencies for seeding...${NC}"
# Using pip with break-system-packages is acceptable on a dedicated single-purpose EC2 host,
# but using a venv is safer. We'll install it globally with the flag since it's just for scripts.
sudo pip3 install psycopg2-binary python-dotenv --break-system-packages || true

echo -e "\n${CYAN}==============================================${NC}"
echo -e "${GREEN}   ✅ Installation Complete!                  ${NC}"
echo -e "${CYAN}==============================================${NC}"
echo -e "${YELLOW}🚨 CRITICAL NEXT STEP:🚨${NC}"
echo -e "Because we added your user to the Docker group, Linux requires you to log out and log back in."
echo -e "1. Type: ${GREEN}exit${NC} and hit Enter (this drops your SSH connection)"
echo -e "2. Press Up Arrow and hit Enter to SSH back into the server."
echo -e "3. Once back in, type: ${GREEN}docker ps${NC} to verify Docker is running without sudo."
echo -e "4. Run: ${GREEN}docker compose -f docker-compose.prod.yml up -d --build${NC}"
