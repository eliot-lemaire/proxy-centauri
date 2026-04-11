# Proxy Centauri — Quick Start Guide

*No coding experience needed. You'll be up and running in under 5 minutes.*

---

## What is Proxy Centauri?

Think of Proxy Centauri as a **traffic director** that sits in front of your website or app.

Instead of people connecting directly to your server, they connect to Proxy Centauri first. It then forwards them to the right place — automatically, instantly, and without them noticing.

**Why would you want this?**

- You have **multiple servers** and want to share the load between them
- You want your site to **stay online** even if one server crashes
- You want to **limit abusive traffic** (bots, scrapers, too many requests)
- You want **HTTPS/SSL** set up without touching your app
- You want to see **live stats** on who's using your service

---

## Before You Start

You need two things installed on your computer or server:

### 1. Docker Desktop
Docker lets you run software in a container — no installation mess, no version conflicts.

- **Mac:** Download from [docker.com/products/docker-desktop](https://www.docker.com/products/docker-desktop/)
- **Windows:** Same link as above
- **Linux:** Run `curl -fsSL https://get.docker.com | sh` in your terminal

After installing, open Docker Desktop and make sure it's running (you'll see the whale icon in your taskbar).

### 2. Git (to download the files)
- **Mac:** Open Terminal, type `git --version`. If it's not installed, macOS will prompt you to install it.
- **Windows:** Download from [git-scm.com](https://git-scm.com/downloads)
- **Linux:** `sudo apt install git` or `sudo yum install git`

---

## Step-by-Step Setup

### Step 1 — Download Proxy Centauri

Open a Terminal (Mac/Linux) or Command Prompt (Windows) and run:

```
git clone https://github.com/eliot-lemaire/proxy-centauri.git
cd proxy-centauri
```

This downloads all the files into a folder called `proxy-centauri`.

---

### Step 2 — Start It Up

```
docker compose up --build -d
```

The first time you run this it will take about 1–2 minutes to download and build everything. After that it starts in seconds.

You'll see output like this — that means it's working:

```
Container goproxy-echo-http-1  Healthy
Container goproxy-echo-tcp-1   Healthy
Container goproxy-centauri-1   Started
```

---

### Step 3 — Check It's Working

Open your web browser and go to:

**http://localhost:8000**

You should see a page of JSON data — that's your backend server responding through the proxy. It's working!

You can also check:
- **Metrics dashboard:** http://localhost:9090/metrics

---

### Step 4 — Point It at Your Own Server

Open the file called `centauri.example.yml` in any text editor (Notepad, TextEdit, VS Code).

Find this section:

```yaml
star_systems:
  - address: "echo-http:3000"
```

Change `echo-http:3000` to your server's address. For example:

```yaml
star_systems:
  - address: "192.168.1.10:3000"   # your local server
```

or

```yaml
star_systems:
  - address: "myapp.com:80"        # a domain name
```

Save the file. Proxy Centauri will detect the change and update automatically — **no restart needed**.

---

## What the Config File Does

The config file (`centauri.example.yml`) is how you tell Proxy Centauri what to do. It uses plain text that's easy to read.

Here's what each part means:

```yaml
jump_gates:             # "jump_gates" = your traffic rules

  - name: "web-app"    # A name for this rule (can be anything)
    listen: ":8000"    # Which port to listen on
    protocol: http     # Type of traffic: http, tcp, or udp

    orbital_router: round_robin  # How to share traffic across servers
                                 # round_robin = take turns evenly
                                 # weighted    = give some servers more traffic
                                 # least_connections = send to least busy server

    flux_shield:                 # Rate limiting (optional)
      requests_per_second: 100   # Max 100 requests per second per visitor
      burst: 20                  # Allow short bursts up to 20 extra

    star_systems:                # "star_systems" = your backend servers
      - address: "server1:3000"  # First server
      - address: "server2:3000"  # Second server (traffic is shared between them)
```

---

## Common Setups

### "I have one website I want to proxy"

```yaml
jump_gates:
  - name: "my-website"
    listen: ":80"
    protocol: http
    star_systems:
      - address: "localhost:3000"   # where your website actually runs
```

### "I have two servers and want to share load"

```yaml
jump_gates:
  - name: "my-app"
    listen: ":8000"
    protocol: http
    orbital_router: round_robin     # alternates between the two
    star_systems:
      - address: "server-a:3000"
      - address: "server-b:3000"
```

### "I want my site to stay up if one server crashes"

Same as above — Proxy Centauri automatically detects crashed servers (within 5 seconds) and stops sending traffic to them. When they come back, it starts using them again. No action needed from you.

### "I want HTTPS on my site"

You need a domain name. Then:

```yaml
jump_gates:
  - name: "secure-site"
    listen: ":443"
    protocol: http
    tls:
      mode: "auto"                    # gets a free certificate automatically
      domain: "yourdomain.com"        # your actual domain
    star_systems:
      - address: "localhost:3000"
```

Free SSL certificate from Let's Encrypt — set up automatically, renewed automatically.

---

## Understanding the Dashboard

Go to **http://localhost:9090/metrics** in your browser to see live stats.

The numbers here follow a format called Prometheus. It looks technical but you can find key info by searching for:

- `centauri_requests_total` — how many requests have been handled
- `centauri_active_connections` — how many visitors are connected right now
- `centauri_errors_total` — any errors (should be 0 in normal operation)

---

## Useful Commands

| What you want to do | Command |
|---------------------|---------|
| Start Proxy Centauri | `docker compose up -d` |
| Stop Proxy Centauri | `docker compose down` |
| See live logs | `docker compose logs -f centauri` |
| Restart after config changes | `docker compose restart centauri` |
| Check if it's running | `docker compose ps` |
| See traffic logs | `docker compose exec centauri tail -f /app/logs/web-app.log` |

---

## Something Went Wrong?

**Nothing at http://localhost:8000**
→ Make sure Docker is running and you started the service with `docker compose up -d`

**Port already in use error**
→ Change the port number in your config. For example change `:8000` to `:8080`

**My backend server isn't being reached**
→ Make sure the address in `star_systems` is correct and the server is actually running.
→ Run `docker compose logs centauri` to see what's happening.

**I edited the config but nothing changed**
→ Save the file again. The watcher should pick it up within a second.
→ If it still doesn't work, run `docker compose restart centauri`

**Need more help?**
→ Open an issue at [github.com/eliot-lemaire/proxy-centauri/issues](https://github.com/eliot-lemaire/proxy-centauri/issues)

---

## Stopping and Cleaning Up

To stop everything:
```
docker compose down
```

To stop and also delete saved data (metrics history, logs):
```
docker compose down -v
```
