# Protocol: x360-gateway

The Xbox 360 security gateway (SG): the secure channel the console opens after its ticket exchange (UDP 3074 behind
an nft redirect to :13075). It terminates the channel, proxies the tunnelled TCP connections of the Live services to their HTTP
backends, answers the user-logon ticket message and the service-26 notification stream, and publishes signed-in users as
presence (namespace `xbox360`) in the OpenPak account core.
