# Goal
Receive inbound requests that other applications or cronjobs can query with a JSON body. This will pick up the data and post a message to the desired slack channel with the message. 

Allow configuration of two emojis, one being investigating, one being resolved. 

JSON should follow

message: {
    channel: XXX
    mentions: XXX
    message: XXX
    reminderIntervals: XXX (default of none)
}
