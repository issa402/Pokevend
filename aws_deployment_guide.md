# AWS Cloud Architecture & Deployment Guide
*From Local Files to Live World-Wide Cloud*

You mentioned you want to skip local testing and go straight to the cloud (AWS EC2 running Ubuntu). This is a great way to learn real Devops and Cloud Infrastructure.

This guide explains **exactly how your code will run in the cloud** and gives you the **exact step-by-step instructions** to make it happen.

---

## Part 1: Deep Dive — How the Cloud Actually Works

### The Fundamental Truth
**There is no "cloud." It's just someone else's computer.**
When you rent an AWS EC2 instance, you are literally just renting a slice of a physical Linux computer sitting in a massive warehouse in Virginia or Ohio. 

Your code doesn't know it's in the cloud. It just thinks it's running on a normal Ubuntu computer.

### How Our Docker Architecture Fits In
If you ran the code *without* Docker, you'd have to install Go, Python, PostgreSQL, Redis, and RabbitMQ directly onto the Ubuntu server. If AWS crashes and you need a new server, you'd have to do it all over again manually.

**This is why we use Docker.**
Docker packages your code AND its environment (the exact Go version, the exact Python libraries) into "Containers." 
When you deploy to AWS, you simply tell Docker: *"Download these containers and run them."*

Here is the flow of data when a user hits your API:

```
[User's Browser / Phone anywhere in the world]
       │
       │ (1) Internet Request to your AWS Public IP (e.g., http://54.12.34.56:3001)
       ▼
[AWS Security Group] 
       │ (2) Acts as a firewall. We will configure it to ONLY allow traffic 
       │     on Port 22 (SSH), Port 80 (HTTP), Port 3001 (Go), Port 8001 (Python).
       ▼
[EC2 Ubuntu Instance]
       │ (3) The traffic hits the Ubuntu server.
       ▼
[Docker Network "pokemontool_default"]
       │ (4) Docker routes the traffic to the correct container based on the port.
       │
       ├─► Port 3001 ──► [Go Server Container] ──► Reads/Writes ──► [PostgreSQL Container]
       │                                       ──► Caches     ──► [Redis Container]
       │
       └─► Port 8001 ──► [Python FastAPI Container] 
                                               ──► Scans eBay
                                               ──► Publishes  ──► [RabbitMQ Container] ──► Read by Go Worker
```

### High-Value Production Security
In the cloud, **bad guys scan every IP address 24/7**. If you expose your database (Port 5432) to the internet, you will be hacked within hours.
Because we are using Docker, PostgreSQL, Redis, and RabbitMQ talk to each other *internally* on a private Docker network. 
In our `docker-compose.prod.yml`, the databases **do NOT expose their ports to the outside world**. Only Go (3001) and Python (8001) are exposed.

---

## Part 2: Step-by-Step AWS Console Setup

Open a browser, log into the AWS Console, and follow these exact steps to rent your server.

### 1. Launch the EC2 Instance
1. Go to **EC2** → Click **Launch instances**.
2. **Name:** `PokémonTool-Prod`
3. **Application and OS Images (AMI):** Select **Ubuntu**, then choose `Ubuntu Server 24.04 LTS (HVM)`.
4. **Instance Type:** 
   - `t2.micro` or `t3.micro` (Free tier eligible, but 1GB RAM is a bit tight for 5 containers).
   - *Recommendation:* `t3.small` (2GB RAM) is much safer so the server doesn't crash, but it costs ~$15/month. If you use `t2.micro`, it might crash if Python does heavy data processing.
5. **Key pair (login):** 
   - Click **Create new key pair**.
   - Name it `pokemontool-key`.
   - Format: `.pem` (if using Mac/Linux terminal or Git Bash on Windows).
   - Click **Create**. *It will download to your computer. DO NOT LOSE THIS FILE.*

### 2. Configure the Security Group (The Firewall)
Under **Network settings**, click **Edit**.
1. Ensure **Auto-assign public IP** is **Enable**.
2. **Firewall (security groups):** Choose **Create security group**. Name it `pokemontool-sg`.
3. Add the following **Inbound rules**:
   - **Type:** SSH (Port 22), **Source:** `Anywhere-IPv4` 0.0.0.0/0 (Lets you connect from terminal).
   - **Type:** Custom TCP, **Port Range:** `3001`, **Source:** `Anywhere-IPv4` 0.0.0.0/0 (Your Go API).
   - **Type:** Custom TCP, **Port Range:** `8001`, **Source:** `Anywhere-IPv4` 0.0.0.0/0 (Your Python API).
4. Click **Launch instance** at the bottom right.

---

## Part 3: Deploying from your Terminal (Windows)

We need to send your code from your Windows PC up to the AWS Ubuntu server.
Because your code is local, we will use Git to put it on GitHub, then pull it down onto the EC2 server.

### Step 1: Push your local code to GitHub
Open your Windows terminal (`C:\Users\isjim\OneDrive\Desktop\Pokemon`):
```bash
git init
git add .
git commit -m "Initial commit for production"
# Go to github.com, create a new PUBLIC repository called "Pokemon"
git branch -M main
git remote add origin https://github.com/YOUR_GITHUB_USERNAME/Pokemon.git
git push -u origin main
```
*Now your code is safely stored in the cloud repository.*

### Step 2: Connect to your EC2 Instance
Go to AWS EC2 Console, click your instance, and copy the **Public IPv4 address** (e.g., `54.123.45.67`).
In your Windows terminal, navigate to where your `.pem` key downloaded (usually Downloads):
```bash
cd ~/Downloads
# In Windows, we have to SSH. If using Git Bash or PowerShell:
ssh -i "pokemontool-key.pem" ubuntu@YOUR_EC2_PUBLIC_IP

# If it asks "Are you sure you want to continue connecting?", type "yes" and hit enter.
```
*You are now inside the Linux server in the cloud! Notice your terminal prompt changed to `ubuntu@ip-172-31-xx-xx`.*

### Step 3: Install Docker and Download Your Code
I have written an automation script that does all the heavy lifting. Run these exact commands inside your EC2 terminal:

```bash
# 1. Download your code from GitHub onto the server
git clone https://github.com/YOUR_GITHUB_USERNAME/Pokemon.git
cd Pokemon

# 2. Make the setup script executable and run it
chmod +x scripts/aws_ec2_setup.sh
./scripts/aws_ec2_setup.sh
```
*This script will install Docker, Docker Compose, and configure permissions. 
**IMPORTANT:** The script will tell you to log out and log back in. Type `exit`, hit enter, and then run the `ssh` command from Step 2 again.*

### Step 4: Start the Environment in Production
Once you have logged back in via SSH:
```bash
cd Pokemon

# We use the PRODUCTION docker-compose file I just created for you.
# The -d flag runs it in the background (detached), so if you close your terminal
# the server keeps running forever.
docker compose -f docker-compose.prod.yml up -d --build

# Wait about 60 seconds for Go and Python to finish compiling inside the containers,
# and for the databases to initialize.

# Apply the database structure (Migrations)
docker exec -i pokemontool_postgres psql -U pokemontool_user -d pokemontool \
  < database/migrations/001_init.sql

# Seed the database with fake data so the API isn't empty
python3 scripts/seed_cards.py
```

---

## Part 4: Using Your Live Cloud APIs!

Open your normal web browser on your computer. Your APIs are now live to the world!

Check if Go is running:
```
http://YOUR_EC2_PUBLIC_IP:3001/health
```

Check if Python is running:
```
http://YOUR_EC2_PUBLIC_IP:8001/health
```

Check your Python API documentation (auto-generated by FastAPI):
```
http://YOUR_EC2_PUBLIC_IP:8001/docs
```

To fetch deals from your Go database remotely:
```
http://YOUR_EC2_PUBLIC_IP:3001/api/deals
```

### How to Monitor the Cloud Server

If something breaks, here is how you debug like a Senior Engineer:

**Check if containers are running:**
```bash
docker ps
```

**Read the live Go logs:**
```bash
docker logs -f pokemontool_go
```

**Read the live Python logs:**
```bash
docker logs -f pokemontool_api_consumer
```

**Restart the whole system if it gets stuck:**
```bash
docker compose -f docker-compose.prod.yml down
docker compose -f docker-compose.prod.yml up -d
```
