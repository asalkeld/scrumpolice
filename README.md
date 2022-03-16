```
                                           _ _
 ___  ___ _ __ _   _ _ __ ___  _ __   ___ | (_) ___ ___
/ __|/ __| '__| | | | '_ ` _ \| '_ \ / _ \| | |/ __/ _ \
\__ \ (__| |  | |_| | | | | | | |_) | (_) | | | (_|  __/
|___/\___|_|   \__,_|_| |_| |_| .__/ \___/|_|_|\___\___|
                              |_|
```

Scrum bot ask every member of your team for a scrum report then reports it to
your team's channel at a predefined time.

- [Jason Fried's - Status meetings are the scourge](https://m.signalvnoise.com/status-meetings-are-the-scourge-39f49267ca90) started all the fuzz.

# Usage

Run the bot with a slack bot user token

```sh
echo xoxb-mytoken > .token
nitric up -s aws
```

Create your configuration file:

```json
{
  "timezone": "America/Montreal",
  "teams": [
    {
      "channel": "themostaswesometeamchannel",
      "name": "L337 team",
      "members": [
        "gfreeman",
        "evance",
        "wbreen"
      ],
      "split_report": true,
      "questions": [
        "What did you do yesterday?",
        "What will you do today?",
        "Are you being blocked by someone for a review? who ? why ?",
        "How will you dominate the world"
      ],
    }
  ]
}
```

`split_report`: whether to post each scrum entry as a separate message or post all scrum entries in the same message.

