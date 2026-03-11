## TODOs

- HTTPClient ReAuth, Retry Func

- UnMarshall url params from struct to url.Values ✅
    - Support for other types than just strings

- flags loader for config variables

- OnShutDown functions to save current state on shutdown ✅
    - expand to OnStart (unsure if this is necessary if handled correctly when registering services)

- AUTH

- Webhooks notifies on event?

- dnsdist ping (for grafana dashboard connectivity?)

- handle script change on HTTP response validation

- in an override situation
    - dnsdist override should not happen from operator, but CLI either from operator pod, or from client
    - operator should flag service with override in storage
    - on override deletion, operator compares with dnsdist if the override reflects the active in service group
        - make appropriate changes if not
    - think of fix for the dnsdist sync job!

- CLI
    inside deployment pod for doing admin tasks