# Mattermost inclusive bot

Bot to help people discontinue harmful terms in their communication.

When invite to a channel, the bot will monitor for harmful terms ([word_list.json](/app/word_list.json)) and will DM a user with suggestions when a term is used.

## Set up

### Create the bot account

As a System Admin account:

1. Go to <https://\<your-mattemost-url\>/\<your-mattermost-team\>/integrations/bots>.
1. Click `Add Bot Account`.
1. Set the bot name as `inclusive-bot` or anything else.
1. Enable the `post:all` option.
1. Save, and keep the provided token.
1. Go to <https://\<your-mattemost-url\>/\<your-mattermostteam\>/messages>.
1. Click `Invite members` and invite your bot.
1. The bot will be invited to your default channels but you can invite it to others too.

The firt time the bot connects it will create a debug channel.

### Configuration

The bot can be configured through a `config.json` file or setting environment variables.

#### Env Variables

The necessary variables are `SITE_URL`, `BOT_TOKEN`, `TEAM_NAME`

##### SITE_URL

Your mattermost address including schema and port(if a non-default port).
E.g.:

- <http://localhost:8066>
- <https://mymattermost.com>

##### BOT_TOKEN

The token provided when creating the bot account in mattermost

##### TEAM_NAME

Your mattermost team name.

#### Config.json file

Rename the `config.json.example` file to `config.json` and change the variables `site_url`, `bot_name`, `team_name` according to your needs.

## Running the bot

### Docker

Build the image:

```
docker build . -t inclusive-bot:dev
```

Run the container:

```
docker run --name=inclusive-bot --network=dev_default --env SITE_URL=VALUE1 --env BOT_TOKEN=VALUE2 --env TEAM_NAME=VALUE3 inclusive-bot
```

### Locally

Set the environment variables:

```
export SITE_URL=VALUE1
export BOT_TOKEN=VALUE2
export TEAM_NAME=VALUE3
```

And start the bot:
`cd app; go run .`
