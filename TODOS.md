## TODOs

- Test to only test server-side errors, or network errors from operator. So a "failing" health-check only is considered a fail if everything is "correct" from the operators perspective

- dnsdist parse show rules response into struct/model.Spoof

- request builder ✅

- HTTPClient ReAuth, Retry Func

- complete dns/updater feat ✅

- UnMarshall url params from struct to url.Values ✅
    - Support for other types than just strings

- flags loader for config variables

- zone-fetcher panics when ctrl+c while doing a zone transfer

- relation to what datacenter the operator runs in, to only create dns spoofs on the service running in that datacenter in active/active

- rewrite servicemanager update existing service

- define constants for different check-types!

- FIX: Service group promotions:
    active and passive are up.
    active goes down -> passive failover
    passive goes down -> no one to failover
    active comes up -> both are checked at same interval afterwards.
        FIX: service group needs to change the way active is handled, and promotion events also propably needs to be changed aswell
    this happends no matter what after a failover

    ActiveActive:
        - no promotion events are triggered