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
With this I assume you know how docker works. If not, google is your friend :)

At present moment, since this repo is private, you will need to add your github username and Personal Access Token to docker. [Follow this guide on github to authenticate on docker](https://docs.github.com/en/packages/working-with-a-github-packages-registry/working-with-the-container-registry#authenticating-with-a-personal-access-token-classic).

Here is an example docker compose file:
```yml
services:
  scratchcord-server:
    image: ghcr.io/preloading/scratchcord-server-rewritten:main
    ports:
      - "8080:3000"
    volumes:
      - '/path/to/scratchcord-server-rewritten/sqlite:/config/sqlite'
```
### Environment Variables (-e)

|Env|Function|Default|
| :----: | --- | :---: |
|``SCRATCHCORD_MOTD``| Sets the message that is sent to every client on login|``"Welcome to scratchcord!"``|
|``SCRATCHCORD_DB_PATH``| Changes the path in the container where the SQLite DB is stored. |``"/config/sqlite/scratchcord.db"``|
|``SCRATCHCORD_WEBHOOK_URL``| Sets the webhook integration to the "general" channel|None|

### 🖥 Bare metal
#### Clone the repo
```bash
git clone https://github.com/Preloading/scratchcord-server-rewritten.git
```
#### Create .env
Copy .env-example as .env & set it to your prefered settings
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
## Credits
@Preloading I made this server!

@Ikelene He made the scratchcord original client

You can find the original client here: https://github.com/Ikelene/Scratchcord