# go-botmetrics

go-botmetrics is a Go client to the
[BotMetrics](https://getbotmetrics.com) service which lets you collect
&amp; analyze metrics for your bot.

[![Build
Status](https://travis-ci.org/botmetrics/go-botmetrics.svg?branch=master)](https://travis-ci.org/botmetrics/go-botmetrics)

## Usage

Log in to your [BotMetrics](https://getbotmetrics.com) account, navigate
to "Bot Settings" and find out your Bot ID and API Key.

With that, you can initialize a `BotmetricsClient`:

```go
import "github.com/botmetrics/go-botmetrics"

client := botmetrics.NewBotmetricsClient("api-key", "bot-id")
```

Alternatively, you can set the following ENV variables

- `ENV['BOTMETRICS_API_KEY']`
- `ENV['BOTMETRICS_BOT_ID']`

and initialize a `BotmetricsClient` with the default ENV variables:

```go
import "github.com/botmetrics/go-botmetrics"

client := botmetrics.NewBotmetricsClient()
```

### `RegisterBot`

With a `BotmetricsClient` instance,
every time you create a new Slack Bot (in the OAuth2 callback),
and assuming the bot token you received as part of the OAuth2 callback is `bot-token`,
you can make the following call:

```go
import "github.com/botmetrics/go-botmetrics"

client := botmetrics.NewBotmetricsClient("api-key", "bot-id")
client.RegisterBot('bot-token', 0)
```

#### Retroactive Registration

If you created your bot in the past, you can pass in `created_at` with
the UNIX timestamp of when your bot was created, like so:

```go
import "github.com/botmetrics/go-botmetrics"

client := botmetrics.NewBotmetricsClient("api-key", "bot-id")
client.RegisterBot('bot-token', 1462318092)
```

## Contributing

Bug reports and pull requests are welcome on GitHub at https://github.com/botmetrics/go-botmetrics. This project is intended to be a safe, welcoming space for collaboration, and contributors are expected to adhere to the [Contributor Covenant](http://contributor-covenant.org) code of conduct.


## License

The gem is available as open source under the terms of the [MIT License](http://opensource.org/licenses/MIT).
