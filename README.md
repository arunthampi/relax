## Relax

Relax is a Message Broker for Slack Bots. What does that mean?
If you are running a "bot-as-a-service" for Slack, you have to maintain
hundreds (if not thousands) of websocket connections and handle the
deluge of events from all these connections.

Relax does all that heavy
lifting for you and provides you with a single stream of events that
your web app can then take action on. The protocol is JSON based and so
any web app can communicate with Relax.

[![Travis Badge for Relax](https://travis-ci.org/zerobotlabs/relax.svg?branch=master)](https://travis-ci.org/zerobotlabs/relax)
[![Gitter](https://badges.gitter.im/zerobotlabs/relax.svg)](https://gitter.im/zerobotlabs/relax?utm_source=badge&utm_medium=badge&utm_campaign=pr-badge)

If you are a Rails app however, there is a nifty [Ruby
client](https://github.com/zerobotlabs/relax-rb) for you to use.

You can also download pre-built binaries [here](#installation).

If you are a Heroku user, you can deploy Relax right away with one click.

[![Deploy](https://www.herokucdn.com/deploy/button.svg)](https://heroku.com/deploy)


**Known Issue:** When deploying using the "Deploy with Heroku" button,
Heroku's Free Redis server takes a while to boot up, so your Relax
deployment will take a while to come up as well. You can see Relax
trying to connect to Heroku's Redis server by tailing logs `heroku logs
--tail -a <your-heroku-app-name>`. After a while, it should connect.

## Coming Soon

* Pluggable Messaging Backends

## Installation

Although Relax is written in Go, it does not require any knowledge
of Go. You can download the latest stable version of relax
[here](https://dl.equinox.io/zerobotlabs/relax/stable).

Binaries are available for both OS X and Linux. Untar and gunzip the
downloaded file and move the `relax` binary to your `$PATH` to start
using.

## Don't Want to Host It Yourself?

If you don't want to host Relax yourself, head on over to
[https://getbotmetrics.com](https://getbotmetrics.com) which offers
Relax hosting as well as analytics and metrics for your Slack bot. You
will receive Relax events as JSON strings to a webhook you specify and
also get analytics and metrics for your Slack bot.


## In Production Use

Relax is used in production to power [Nestor](https://www.asknestor.me).

## Running Relax

To run it, basically run `relax` (assuming it is in your $PATH).

`RELAX_BOTS_KEY=relax_bots_key RELAX_BOTS_PUBSUB=relax_bots_pubsub RELAX_EVENTS_QUEUE=relax_events_queue REDIS_HOST=localhost:6379 relax`

## Setup

The Relax message broker requires a few environment variables to be set up (these same environment variables are also used to set up the Relax Ruby Client). These environment variables are basically Redis keys that can be configured based on your specific needs.

`RELAX_BOTS_KEY`: This can be any string value and is used to store state about all Slack clients currently controlled by Relax in Redis.

`RELAX_BOTS_PUBSUB`: This can be any string value and is used by Relax clients to notify Relax brokers that a new Slack bot has been started.

`RELAX_EVENTS_QUEUE`: This can be any string value and is used by Relax brokers to send events to the client.

`RELAX_MUTEX_KEY`: This can be any string value and is used by Relax brokers to decide whether to send events back to clients.

## Protocol

You interact with Relax by sending messages to Relax via Redis, there
are two primary ways of interacting with Relax:

### Starting Bots

To start a bot, you need to `HSET` on `RELAX_BOTS_KEY` with a JSON blob
containing `"team_id"` and `"token"` keys which represent the Team UID
and Token for the bot you want to start. Along with this, you should
also `PUBLISH` on `RELAX_BOTS_PUBSUB` with a JSON blob containing the
keys `"type"` and `"team_id"` containing the values `"team_added"` and
the Team UID of the bot you want to start.

For e.g., for a bot who's Slack Team UID is "TDEADBEEF" and who's token
is "xoxo_slackbotoken", you can issue the following commands using
`redis-cli` to start a bot (assuming that `$RELAX_BOTS_KEY` is `relax_bots_key`):

```bash
$ redis-cli
127.0.0.1:6379> MULTI
OK
127.0.0.1:6379> HSET relax_bots_key TDEADBEEF '{"team_id":"TDEADBEEF","token":"xoxo_slackbotoken"}'
(integer) 1
127.0.0.1:6379> HGETALL relax_bots_key
1) "TDEADBEEF"
2) "{\"team_id\":\"TDEADBEEF\",\"token\":\"xoxo_slackbotoken\"}"
127.0.0.1:6379> PUBLISH relax_bots_pubsub '{"type":"team_added","team_id":"TDEADBEEF"}'
QUEUED
127.0.0.1:6379> EXEC
```

### Listening for Events

Relax also generates events (details of events are described in the ["Events" section of the README](https://github.com/zerobotlabs/relax#events))

Events are queued in the `$RELAX_EVENTS_QUEUE` key in Redis and so to consume events,
you need to `LPOP` or `BLPOP` the `$RELAX_EVENTS_QUEUE` to deal with events.

## Events

Slack Events are gathered from all teams that Slack is
listening to and are multiplexed onto a single Redis queue. The event
data structure consists of the following fields:

### type

This is a string value contains the type of event can hold the following values:

Type               | What it does
-------------------|---------------
`disable_bot`      | This event is sent when authentication with a team fails (either due to a wrong token or an expired token).  `message_new`    | This is event is sent When a new message is received by Relax. *Note*: Only events for messages intended to Relax (so an @-mention to the bot or a direct message) are sent.
`message_changed`  | This event is sent when a message has been edited.
`message_deleted`  | This event is sent when a message has been deleted.
`reaction_added`   | This event is sent when a reaction has been added to a message.
`reaction_removed` | This event is sent when a reaction has been removed from a message.
`team_joined`      | This event is sent when a new member has been added to the team. The best practice upon receiving this event is to refresh the team database and make sure that information on all members of the team is up to date.
`im_created`       | This event is sent when a new direct message has been opened with the bot. This can be ignored in most cases as it is used by Relax to keep internal metadata in sync.

### user_uid

This is a string value and is the UID (generated by Slack) of the user associated with the
event. So in the case of `message_new` it's the UID of the user who
created the message, for `reaction_added`, it's the UID of the user who added a reaction to a message.

### channel_uid

This is the UID (generated by Slack) of the channel associated with the event.

### im

This is a boolean value to indicate whether the channel (with the UID
`channel_uid`) is an IM or not.

### text

This is a string value and contains the text associated with the
event. This can mean different things in different contexts:

Type               | What "text" field means
-------------------|---------------
`message_new`      | The message text
`message_changed`  | The *new* text of the message
`message_deleted`  | The original message text
`reaction_added`   | Reaction added to a message. This contains the text representation of a reaction, for e.g. `:simple_smiley:`
`reaction_removed` | Reaction removed from a message. This contain the text representation of a reaction, for e.g. `:simple_smiley:`
`team_joined`      | Since there is no text metadata associated with this event, it is always blank.
`im_created`       | Since there is no text metadata associated with this event, it is always blank.

### relax_bot_uid

This is a string value and represent the UID of the bot that Relax
controls. This is useful if you want to strip out the @-mention word in
a message @-mention'ed to the bot.

### timestamp

This is a string value and represents the timestamp at which a
particular event occurred but can have different meanings in different
contexts:

Type               | What "timestamp" field means
-------------------|---------------
`disable_bot`      | Empty string
`message_new`      | Timestamp at which the message was created. In this case, `timestamp` and `event_timestamp` will be the same
`message_changed`  | Timestamp of the message that has been changed. Upon receiving a "message_changed" event, you can use "channel_uid" and "timestamp" to identify the message who's text has been changed and modify its text accordingly
`message_deleted`  | Timestamp of the message that has been deleted. Upon receiving a "message_changed" event, you can use "channel_uid" and "timestamp" to identify the message that has been deleted and delete that message accordingly
`reaction_added`   | Timestamp of the message for which a reaction has been added. Upon receiving a "reaction_added" event, you can use "channel_uid" and "timestamp" to identify the message for which a reaction has been added and change the metadata for that message accordingly
`reaction_removed` | Timestamp of the message for which a reaction has been removed. Upon receiving a "reaction_removed" event, you can use "channel_uid" and "timestamp" to identify the message for which a reaction has been removed and change the metadata for that message accordingly
`team_joined`      | Empty string
`im_created`       | Empty string

### provider

This is a string value and until Relax supports multiple providers,
this is always "slack".

### event_timestamp

This is a string value and represents the time at which an event occurs.
In the case of `disable_bot`, `team_joined` and `im_created` events, it is
an empty string.
