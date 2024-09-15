# Scratchcord Server V2
##### This time not written in scratch!

## What is this?
A friend of mine wrote an app called "Scratchcord", which is more similar to IRC than anything else. I noticed how he was using a library called Cloudlink, on publicly accessble nodes, with a "server" running as a client, where anyone could intercept messages & passwords being sent in **plaintext** for anyone to see! Because of this, I took some pitty and decided to write this! It's also a way for me to dip my toes in Go.

## Features
- 🔒 Any proper security (eg. passwords aren't stored in plaintext)
- 🚀 Fast
- 🐳 Easy to deploy
- 💾 Persistant Message & Account storage
- ...and much more!

## Setup
Setting this up, you can take two paths.

- 🖥 Bare metal
- 🐳 Docker

### 🐳 Docker (recommended)
Instructions TODO
### 🖥 Bare metal
#### Clone the repo
```bash
git clone https://github.com/Preloading/scratchcord-server-rewritten.git
```
#### Building
```bash
go build .
```
#### Run the executable
🪟 Windows
```powershell
.\scratchcord-server.exe
```
🐧 Linux
```bash
chmod -x scratchcord-server
./scratchcord-server
```
Setting up autostarting, and auto-updating is on you 🙃