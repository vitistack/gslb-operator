## TODOs

- HTTPClient ReAuth, Retry Func

- UnMarshall url params from struct to url.Values ✅
    - Support for other types than just strings

- flags loader for config variables

- OnShutDown functions to save current state on shutdown ✅
    - expand to OnStart (unsure if this is necessary if handled correctly when registering services)

- If svc not in DC, then roundtrip decides priority

- AUTH

- Webhooks notifies on event?

- worker pool stats handling from manager